package monitor

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestDemoPreviewProtectsAgentRoot(t *testing.T) {
	service := NewService()
	plan, err := service.Preview("demo-codex-old-branch", true)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	if len(plan.Targets) != 12 {
		t.Fatalf("want 12 MCP targets, got %d", len(plan.Targets))
	}
	if len(plan.Protected) != 1 || plan.Protected[0].PID != 61201 {
		t.Fatalf("want protected Codex root 61201, got %#v", plan.Protected)
	}
	for _, target := range plan.Targets {
		if target.PID == 61201 {
			t.Fatal("agent root must never be an action target")
		}
		if target.StartTimeMs <= 0 {
			t.Fatalf("target %d has no start-time identity", target.PID)
		}
	}
	if plan.EstimatedRSS != 3_048*MB {
		t.Fatalf("want 3.048 GB target RSS, got %d", plan.EstimatedRSS)
	}
	if len(plan.Token) != 36 || plan.CreatedAtUnix <= 0 || plan.ExpiresAtUnix-plan.CreatedAtUnix != 120 {
		t.Fatalf("invalid one-time plan token/window: %+v", plan)
	}
	if len(plan.Warnings) < 3 {
		t.Fatalf("expected explicit safety warnings, got %#v", plan.Warnings)
	}
}

func TestDemoPreviewRejectsUnsafeOrUnknownWorkloads(t *testing.T) {
	service := NewService()
	if _, err := service.Preview("demo-vite", true); err == nil || !strings.Contains(err.Error(), "足够证据") {
		t.Fatalf("protected workload should be rejected, got %v", err)
	}
	if _, err := service.Preview("missing", true); err == nil || !strings.Contains(err.Error(), "不存在") {
		t.Fatalf("unknown workload should be rejected, got %v", err)
	}
}

func TestPreviewTokenCannotExecuteBeforeConfirmation(t *testing.T) {
	service := NewService()
	plan, err := service.Preview("demo-codex-old-branch", true)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}

	if _, err := service.Execute(plan.Token); err == nil || !strings.Contains(err.Error(), "未经确认") {
		t.Fatalf("preview token must never execute directly, got %v", err)
	}

	snapshot, err := service.Snapshot(true)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	for _, workload := range snapshot.Workloads {
		if workload.ID == plan.WorkloadID {
			return
		}
	}
	t.Fatal("preview-only execution must not reclaim the demo workload")
}

func TestDemoExecuteAfterExplicitConfirmationConsumesBothTokensOnce(t *testing.T) {
	service := NewService()
	plan, err := service.Preview("demo-codex-old-branch", true)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	confirmation, err := service.Confirm(plan.Token)
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	if confirmation.Token == "" || confirmation.Token == plan.Token {
		t.Fatalf("confirmation must use a fresh execution token: %+v", confirmation)
	}
	if confirmation.TargetCount != len(plan.Targets) || confirmation.EstimatedRSS != plan.EstimatedRSS {
		t.Fatalf("confirmation summary does not match preview: %+v", confirmation)
	}

	result, err := service.Execute(confirmation.Token)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if len(result.Stopped) != 12 || len(result.Failed) != 0 || result.ReclaimedBytes != plan.EstimatedRSS {
		t.Fatalf("unexpected action result: %+v", result)
	}
	if result.ConfirmedAt <= 0 {
		t.Fatalf("result must include confirmation time: %+v", result)
	}
	if _, err := service.Execute(confirmation.Token); err == nil {
		t.Fatal("reusing an execution token must fail")
	}
	if _, err := service.Confirm(plan.Token); err == nil {
		t.Fatal("reusing a preview token after confirmation must fail")
	}

	snapshot, err := service.Snapshot(true)
	if err != nil {
		t.Fatalf("snapshot failed: %v", err)
	}
	for _, workload := range snapshot.Workloads {
		if workload.ID == plan.WorkloadID {
			t.Fatal("reclaimed demo workload should disappear from next snapshot")
		}
	}
	if len(snapshot.Events) == 0 || snapshot.Events[0].Kind != "reclaimed" {
		t.Fatalf("expected audit event, got %#v", snapshot.Events)
	}
}

func TestExpiredPreviewCannotBeConfirmed(t *testing.T) {
	service := NewService()
	plan, err := service.Preview("demo-codex-old-branch", true)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	service.mu.Lock()
	pending := service.pending[plan.Token]
	pending.createdAt = time.Now().Add(-3 * time.Minute)
	service.pending[plan.Token] = pending
	service.mu.Unlock()

	if _, err := service.Confirm(plan.Token); err == nil || !strings.Contains(err.Error(), "过期") {
		t.Fatalf("expired preview should be rejected, got %v", err)
	}
}

func TestExpiredConfirmationCannotExecute(t *testing.T) {
	service := NewService()
	plan, err := service.Preview("demo-codex-old-branch", true)
	if err != nil {
		t.Fatalf("preview failed: %v", err)
	}
	confirmation, err := service.Confirm(plan.Token)
	if err != nil {
		t.Fatalf("confirm failed: %v", err)
	}
	service.mu.Lock()
	confirmed := service.confirmed[confirmation.Token]
	confirmed.expiresAt = time.Now().Add(-time.Second)
	service.confirmed[confirmation.Token] = confirmed
	service.mu.Unlock()

	if _, err := service.Execute(confirmation.Token); err == nil || !strings.Contains(err.Error(), "过期") {
		t.Fatalf("expired confirmation should be rejected, got %v", err)
	}
}

func TestSameStartTimeRequiresKnownAndCloseIdentity(t *testing.T) {
	now := time.Now().UnixMilli()
	tests := []struct {
		name     string
		actual   int64
		expected int64
		want     bool
	}{
		{name: "same", actual: now, expected: now, want: true},
		{name: "rounding tolerance", actual: now, expected: now - 1800, want: true},
		{name: "reused pid", actual: now, expected: now - 5000, want: false},
		{name: "missing expected", actual: now, expected: 0, want: false},
		{name: "missing actual", actual: 0, expected: now, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sameStartTime(tt.actual, tt.expected); got != tt.want {
				t.Fatalf("sameStartTime(%d,%d)=%v want %v", tt.actual, tt.expected, got, tt.want)
			}
		})
	}
}

func TestExecuteRejectsPIDIdentityMismatch(t *testing.T) {
	result := executeLive(ActionPlan{
		WorkloadID: "test-workload",
		Targets: []ActionTarget{{
			PID:         int32(os.Getpid()),
			StartTimeMs: 1,
			Name:        "current-test-process",
			RSSBytes:    MB,
		}},
	})
	if len(result.Failed) != 1 || len(result.Stopped) != 0 {
		t.Fatalf("mismatched process identity must fail safely: %+v", result)
	}
	if !strings.Contains(result.Failed[0].Reason, "身份校验失败") {
		t.Fatalf("unexpected failure reason: %q", result.Failed[0].Reason)
	}
}
