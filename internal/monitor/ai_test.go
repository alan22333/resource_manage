package monitor

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func clearAIEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{aiBaseURLEnv, aiModelEnv, aiAPIKeyEnv, aiAllowRemoteEnv} {
		t.Setenv(key, "")
	}
}

func TestLocalAIAnswersCoreDiagnosticQuestions(t *testing.T) {
	clearAIEnvironment(t)
	service := NewService()

	tests := []struct {
		name      string
		question  string
		wantTitle string
	}{
		{name: "pressure", question: "为什么电脑变卡？", wantTitle: "电脑变卡时先看这三个信号"},
		{name: "thermal", question: "为什么电脑发热？", wantTitle: "热压力偏高时先查残留任务"},
		{name: "mcp", question: "哪个任务留下了最多 MCP 后代？", wantTitle: "MCP 后代最多"},
		{name: "reclaim", question: "有什么可以安全回收？", wantTitle: "先处理证据最完整的残留后代"},
		{name: "port", question: "谁占用了 5173 端口？", wantTitle: "端口 5173 归属于"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer, err := service.Explain(tt.question, "demo-codex-old-branch", true)
			if err != nil {
				t.Fatalf("Explain() failed: %v", err)
			}
			if !strings.Contains(answer.Title, tt.wantTitle) {
				t.Fatalf("title %q does not contain %q", answer.Title, tt.wantTitle)
			}
			if len(answer.Facts) == 0 || answer.Inference == "" || answer.Recommendation == "" || answer.Uncertainty == "" {
				t.Fatalf("answer must include all four reasoning sections: %+v", answer)
			}
			if answer.Source != "local-rules" {
				t.Fatalf("want local-rules source, got %q", answer.Source)
			}
		})
	}
}

func TestLocalThermalAnswerUsesPressureInsteadOfInventedTemperature(t *testing.T) {
	clearAIEnvironment(t)
	snapshot := BuildDemoSnapshot(time.Now(), nil)
	answer, err := NewService().Explain("现在温度高吗，是不是后台任务导致的？", "", true)
	if err != nil {
		t.Fatalf("Explain() failed: %v", err)
	}
	combined := strings.Join(append([]string{answer.Title, answer.Inference, answer.Recommendation, answer.Uncertainty}, answer.Facts...), "\n")
	if !strings.Contains(combined, "热压力等级，不是精确温度") {
		t.Fatalf("thermal answer must explain the public API boundary: %+v", answer)
	}
	for _, forbidden := range []string{"°C", "℃", "摄氏度："} {
		if strings.Contains(combined, forbidden) {
			t.Fatalf("thermal answer invented a temperature unit %q: %s", forbidden, combined)
		}
	}
	if !strings.Contains(answer.Recommendation, "回收预览") || !strings.Contains(answer.Recommendation, "不会发送") {
		t.Fatalf("thermal answer should stay read-only and route through preview: %+v", answer)
	}
	_ = snapshot
}

func TestThermalAIContextRedactsSensitiveWorkloadData(t *testing.T) {
	snapshot := thermalTestSnapshot("agent-sensitive")
	snapshot.Thermal = ThermalStatus{
		State:           ThermalSerious,
		Available:       true,
		Active:          true,
		DurationSeconds: 372,
		Cause:           "PID 43210 从 /Users/alan/private 运行 --token sk-action-secret",
		Evidence: []Evidence{
			{Label: "命令行", Detail: "/Users/alan/private/agent --api-key sk-action-secret"},
		},
	}
	snapshot.Workloads[1].Name = "Codex · 会话 43210"
	snapshot.Workloads[1].RiskDetail = "PID 43210 /Users/alan/private/agent --token sk-action-secret"
	snapshot.Workloads[1].LastActivity = "/Users/alan/private/project 43210"
	snapshot.Workloads[1].Evidence = []Evidence{{Label: "命令", Detail: "node /Users/alan/tool.js --token sk-action-secret"}}
	snapshot.Summary.PressureDetail = "PID 43210 /Users/alan/private/agent --token sk-action-secret"
	snapshot.Summary.TopWorkloadName = "Codex · 会话 43210 --api-key sk-action-secret"
	snapshot.Summary.TopWorkloadID = "demo-sensitive-workload-id"
	context := buildAIContext(snapshot)
	payload, err := json.Marshal(context)
	if err != nil {
		t.Fatalf("marshal AI context: %v", err)
	}
	encoded := string(payload)
	if !strings.Contains(encoded, `"thermal"`) {
		t.Fatalf("AI context must include thermal evidence: %s", encoded)
	}
	for _, forbidden := range []string{"43210", "/Users", "sk-action-secret", "demo-sensitive-workload-id", `"pid"`, `"command"`} {
		if strings.Contains(encoded, forbidden) {
			t.Fatalf("AI context leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestLocalAIRejectsInvalidQuestions(t *testing.T) {
	clearAIEnvironment(t)
	service := NewService()

	if _, err := service.Explain("   ", "", true); err == nil || !strings.Contains(err.Error(), "请输入") {
		t.Fatalf("empty question should be rejected, got %v", err)
	}
	if _, err := service.Explain(strings.Repeat("问", 501), "", true); err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatalf("overlong question should be rejected, got %v", err)
	}
}

func TestAIStatusDefaultsToLocalAndRequiresRemoteOptIn(t *testing.T) {
	clearAIEnvironment(t)
	service := NewService()
	status := service.AIStatus()
	if status.Configured || status.Mode != "local-rules" || status.Remote {
		t.Fatalf("unexpected default AI status: %+v", status)
	}

	t.Setenv(aiBaseURLEnv, "https://api.example.com/v1")
	t.Setenv(aiModelEnv, "example-model")
	status = service.AIStatus()
	if status.Configured || status.Mode != "local-rules" || !strings.Contains(status.Privacy, "尚未确认允许发送") {
		t.Fatalf("remote endpoint must stay disabled without opt-in: %+v", status)
	}

	t.Setenv(aiAllowRemoteEnv, "true")
	status = service.AIStatus()
	if status.Configured || status.Mode != "local-rules" || !strings.Contains(status.Privacy, "没有 API Key") {
		t.Fatalf("remote provider without a key must stay on local rules: %+v", status)
	}

	t.Setenv(aiAPIKeyEnv, "test-only-key")
	status = service.AIStatus()
	if !status.Configured || status.Mode != "openai-compatible" || !status.Remote || status.Model != "example-model" {
		t.Fatalf("explicit remote opt-in should enable the provider: %+v", status)
	}
}

func TestAIConnectionDoesNotReturnOrLogCredentials(t *testing.T) {
	const key = "test-secret-that-must-not-leak"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer "+key {
			t.Errorf("unexpected authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"reported-model","choices":[{"message":{"content":"OK"}}]}`))
	}))
	defer server.Close()

	result, err := NewService().TestAIConnection(t.Context(), AIProviderInput{
		Provider:    aiProviderCustom,
		BaseURL:     server.URL + "/v1",
		Model:       "test-model",
		APIKey:      key,
		AllowRemote: true,
	})
	if err != nil {
		t.Fatalf("TestAIConnection() failed: %v", err)
	}
	if !result.OK || result.Model != "reported-model" {
		t.Fatalf("unexpected test result: %+v", result)
	}
	payload, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	if strings.Contains(string(payload), key) {
		t.Fatalf("connection result leaked the API key: %s", payload)
	}
}

func TestAIConnectionFailureDoesNotEchoResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("provider-body-must-not-leak"))
	}))
	defer server.Close()

	_, err := NewService().TestAIConnection(t.Context(), AIProviderInput{
		Provider:    aiProviderCustom,
		BaseURL:     server.URL + "/v1",
		Model:       "test-model",
		APIKey:      "test-only-key",
		AllowRemote: true,
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("expected a status-only error, got %v", err)
	}
	if strings.Contains(err.Error(), "provider-body-must-not-leak") {
		t.Fatalf("connection error echoed the response body: %v", err)
	}
}

func TestInvalidAIProviderDoesNotChangeActiveState(t *testing.T) {
	clearAIEnvironment(t)
	service := NewService()
	if err := service.SetAIProvider(AIProviderInput{
		Provider: aiProviderLocal,
		BaseURL:  "http://127.0.0.1:11434/v1",
		Model:    "local-model",
	}, "test"); err != nil {
		t.Fatalf("set initial provider: %v", err)
	}

	err := service.SetAIProvider(AIProviderInput{
		Provider:    aiProviderCustom,
		BaseURL:     "javascript:alert(1)",
		Model:       "bad-model",
		APIKey:      "test-only-key",
		AllowRemote: true,
	}, "test")
	if err == nil {
		t.Fatal("invalid provider URL should be rejected")
	}
	status := service.AIStatus()
	if status.Model != "local-model" || !status.Configured || status.Remote {
		t.Fatalf("invalid input changed the active provider: %+v", status)
	}
}

func TestClearAIProviderIsIdempotent(t *testing.T) {
	clearAIEnvironment(t)
	service := NewService()
	service.ClearAIProvider()
	service.ClearAIProvider()
	status := service.AIStatus()
	if status.Configured || status.Mode != "local-rules" {
		t.Fatalf("clear should return to local rules: %+v", status)
	}
}

func TestAIContextOmitsProcessIdentityAndCommands(t *testing.T) {
	snapshot := BuildDemoSnapshot(time.Date(2026, 11, 5, 10, 30, 0, 0, time.Local), nil)
	snapshot.Workloads[0].Processes[0].Command = "OPENAI_API_KEY=should-not-leak node secret.js"
	payload, err := json.Marshal(buildAIContext(snapshot))
	if err != nil {
		t.Fatalf("marshal AI context: %v", err)
	}
	text := string(payload)
	for _, forbidden := range []string{"should-not-leak", `"pid"`, `"command"`, `"startTimeMs"`, `"ppid"`} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("AI context leaked %q: %s", forbidden, text)
		}
	}
}

func TestOpenAICompatibleAdapterValidatesAndReturnsStructuredAnswer(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		received = string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"title\":\"模型诊断\",\"facts\":[\"事实一\"],\"inference\":\"推断\",\"recommendation\":\"先预览\",\"uncertainty\":\"采样有限\"}"}}]}`))
	}))
	defer server.Close()

	snapshot := BuildDemoSnapshot(time.Now(), nil)
	answer, err := requestAIAnswer(t.Context(), aiProviderConfig{
		baseURL: server.URL + "/v1",
		model:   "test-model",
	}, buildAIContext(snapshot), "为什么变卡？")
	if err != nil {
		t.Fatalf("requestAIAnswer() failed: %v", err)
	}
	if answer.Title != "模型诊断" || len(answer.Facts) != 1 || answer.Inference != "推断" {
		t.Fatalf("unexpected model answer: %+v", answer)
	}
	for _, forbidden := range []string{"should-not-leak", `"pid"`, `"command"`} {
		if strings.Contains(received, forbidden) {
			t.Fatalf("outbound request leaked %q: %s", forbidden, received)
		}
	}
	if !strings.Contains(received, `"response_format"`) || !strings.Contains(received, `"json_object"`) {
		t.Fatal("outbound request should request JSON mode")
	}
}

func TestOpenAICompatibleAdapterAcceptsFlexibleTextFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{
				"finish_reason": "stop",
				"message": map[string]string{
					"content": `{"title":"兼容格式","facts":"单条事实","inference":["推断一","推断二"],"recommendation":["建议一"],"uncertainty":"采样有限"}`,
				},
			}},
		})
	}))
	defer server.Close()

	answer, err := requestAIAnswer(t.Context(), aiProviderConfig{
		baseURL: server.URL + "/v1",
		model:   "test-model",
	}, buildAIContext(BuildDemoSnapshot(time.Now(), nil)), "为什么变卡？")
	if err != nil {
		t.Fatalf("requestAIAnswer() should accept string/list variants: %v", err)
	}
	if answer.Title != "兼容格式" || len(answer.Facts) != 1 || answer.Inference != "推断一\n推断二" || answer.Recommendation != "建议一" {
		t.Fatalf("unexpected normalized answer: %+v", answer)
	}
}

func TestOpenAICompatibleAdapterRejectsTruncatedCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"finish_reason":"length","message":{"content":"{\"title\":\"未完成\""}}]}`))
	}))
	defer server.Close()

	_, err := requestAIAnswer(t.Context(), aiProviderConfig{
		baseURL: server.URL + "/v1",
		model:   "test-model",
	}, buildAIContext(BuildDemoSnapshot(time.Now(), nil)), "为什么变卡？")
	if err == nil || !strings.Contains(err.Error(), "长度限制") {
		t.Fatalf("truncated completion should fail clearly, got %v", err)
	}
}

func TestExtractJSONObjectHandlesModelWrappers(t *testing.T) {
	want := `{"title":"诊断","facts":["CPU 偏高"],"inference":"可能繁忙","recommendation":"先预览","uncertainty":"采样有限"}`
	for _, input := range []string{
		want,
		"```json\n" + want + "\n```",
		"以下是诊断：\n" + want + "\n请核对。",
	} {
		if got := extractJSONObject(input); got != want {
			t.Fatalf("extractJSONObject() = %q, want %q", got, want)
		}
	}
}

func TestAIFallsBackToLocalRulesWhenModelResponseIsInvalid(t *testing.T) {
	clearAIEnvironment(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"not-json"}}]}`))
	}))
	defer server.Close()
	t.Setenv(aiBaseURLEnv, server.URL)
	t.Setenv(aiModelEnv, "test-model")

	answer, err := NewService().Explain("为什么电脑变卡？", "", true)
	if err != nil {
		t.Fatalf("Explain() should degrade instead of failing: %v", err)
	}
	if answer.Source != "local-rules-fallback" || !strings.Contains(answer.Note, "自动降级") {
		t.Fatalf("expected transparent local fallback: %+v", answer)
	}
}

func TestAIAnswerHasNoExecutableFields(t *testing.T) {
	answer := AIAnswer{
		Title:          "test",
		Facts:          []string{"fact"},
		Inference:      "inference",
		Recommendation: "recommendation",
		Uncertainty:    "uncertainty",
		Source:         "local-rules",
	}
	payload, err := json.Marshal(answer)
	if err != nil {
		t.Fatalf("marshal answer: %v", err)
	}
	for _, forbidden := range []string{"actionToken", "command", "shell", `"pid"`} {
		if strings.Contains(string(payload), forbidden) {
			t.Fatalf("AI answer exposed executable field %q: %s", forbidden, payload)
		}
	}
}
