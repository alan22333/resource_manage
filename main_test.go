package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/aitaskmanager/ai-task-manager/internal/monitor"
)

func newRestoreTestManager(t *testing.T, baseURL string, readSecret func(string) ([]byte, error)) *TaskManager {
	t.Helper()
	dir := t.TempDir()
	settingsPath := filepath.Join(dir, "ai-provider.json")
	saved := persistedAIProvider{
		Provider:    "openai-compatible",
		BaseURL:     baseURL,
		Model:       "test-model",
		AllowRemote: false,
	}
	data, err := json.Marshal(saved)
	if err != nil {
		t.Fatalf("marshal settings: %v", err)
	}
	if err := os.WriteFile(settingsPath, data, 0o600); err != nil {
		t.Fatalf("write settings: %v", err)
	}
	return &TaskManager{
		service:      monitor.NewService(),
		settingsPath: settingsPath,
		readSecret:   readSecret,
	}
}

func TestAIStatusReportsCredentialRestoreInProgress(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	manager := newRestoreTestManager(t, "http://127.0.0.1:1/v1", func(string) ([]byte, error) {
		close(started)
		<-release
		return []byte("test-key"), nil
	})

	status := manager.AIStatus()
	if !status.Restoring {
		t.Fatalf("expected restoring status during background credential load: %+v", status)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("background restore did not reach the secret reader")
	}
	if manager.waitForAIRestore(20 * time.Millisecond) {
		t.Fatal("waitForAIRestore should time out while the credential reader is stalled")
	}
	close(release)
	if !manager.waitForAIRestore(time.Second) {
		t.Fatal("restore should complete after the credential reader returns")
	}
	status = manager.AIStatus()
	if status.Restoring || !status.Configured || !status.APIKeyAvailable {
		t.Fatalf("restored status is not ready: %+v", status)
	}
}

func TestExplainWaitsForRestoredCredential(t *testing.T) {
	requestSeen := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestSeen <- r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"content":"{\"title\":\"模型诊断\",\"facts\":[\"CPU 偏高\"],\"inference\":\"可能繁忙\",\"recommendation\":\"先预览安全回收\",\"uncertainty\":\"采样有限\"}"}}]}`)
	}))
	defer server.Close()

	manager := newRestoreTestManager(t, server.URL+"/v1", func(string) ([]byte, error) {
		time.Sleep(120 * time.Millisecond)
		return []byte("test-restored-key"), nil
	})
	if status := manager.AIStatus(); !status.Restoring {
		t.Fatalf("expected restore to start in background: %+v", status)
	}

	started := time.Now()
	answer, err := manager.Explain("为什么电脑变卡？", "", true)
	if err != nil {
		t.Fatalf("Explain() failed: %v", err)
	}
	if elapsed := time.Since(started); elapsed < 100*time.Millisecond {
		t.Fatalf("Explain() returned before credential restore completed: %s", elapsed)
	}
	if answer.Source != "openai-compatible" || answer.Model != "test-model" {
		t.Fatalf("expected model answer after restore, got %+v", answer)
	}
	select {
	case auth := <-requestSeen:
		if !strings.HasPrefix(auth, "Bearer ") {
			t.Fatalf("model request did not use restored credential")
		}
	case <-time.After(time.Second):
		t.Fatal("model request was not observed")
	}
}
