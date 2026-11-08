package monitor

import (
	"strings"
	"testing"
	"time"
)

func thermalTestSnapshot(causeID string) Snapshot {
	if causeID == "" {
		causeID = "agent-residual"
	}
	return Snapshot{
		System: SystemMetrics{
			CPUPercent:     61.2,
			MemoryUsed:     12 * GB,
			MemoryTotal:    24 * GB,
			SwapUsed:       900 * MB,
			ContainerCount: 1,
		},
		Workloads: []Workload{
			{
				ID:           "dev-server",
				Name:         "Vite · localhost:5173",
				Kind:         "server",
				CPUPercent:   8,
				RSSBytes:     320 * MB,
				ProcessCount: 3,
				Risk:         "healthy",
				Confidence:   92,
			},
			{
				ID:           causeID,
				Name:         "Codex · feature/old-branch",
				Kind:         "agent",
				CPUPercent:   4.2,
				RSSBytes:     3 * GB,
				ProcessCount: 13,
				AgeSeconds:   47 * 60,
				Risk:         "reclaim",
				RiskLabel:    "疑似残留",
				RiskDetail:   "旧分支仍保留重复 MCP 后代",
				Confidence:   96,
				Reclaimable:  true,
				LastActivity: "47 分钟前",
				Processes:    make([]ProcessNode, 13),
			},
		},
	}
}

func TestThermalUnavailableIsReportedWithoutInventingTemperature(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	status := tracker.observe(thermalReading{state: ThermalUnavailable}, thermalTestSnapshot(""), now)

	if status.State != ThermalUnavailable || status.Available || status.Active {
		t.Fatalf("unexpected unavailable status: %+v", status)
	}
	if !strings.Contains(status.Recommendation, "不要猜测具体温度") {
		t.Fatalf("unavailable status must explain the API boundary: %+v", status)
	}
	if status.StartedAt != 0 {
		t.Fatalf("inactive status must not expose a zero-time timestamp: %d", status.StartedAt)
	}
}

func TestThermalFairStartsEpisodeWithoutAlert(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	status := tracker.observe(thermalReading{state: ThermalFair, available: true}, thermalTestSnapshot(""), now)

	if !status.Active || !status.Available || status.State != ThermalFair || status.StartedAt != now.UnixMilli() {
		t.Fatalf("fair state should start a tracking episode: %+v", status)
	}
	if alert := tracker.takeAlert(); alert != nil {
		t.Fatalf("fair state must not notify: %+v", alert)
	}
}

func TestThermalSeriousDelayStartsWhenSeriousBegins(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	start := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	snapshot := thermalTestSnapshot("")
	tracker.observe(thermalReading{state: ThermalFair, available: true}, snapshot, start)

	seriousAt := start.Add(10 * time.Minute)
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, seriousAt)
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, seriousAt.Add(59*time.Second))
	if alert := tracker.takeAlert(); alert != nil {
		t.Fatalf("serious state alerted before 60 continuous seconds: %+v", alert)
	}

	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, seriousAt.Add(60*time.Second))
	alert := tracker.takeAlert()
	if alert == nil || alert.State != ThermalSerious {
		t.Fatalf("serious state should alert after 60 seconds, got %+v", alert)
	}

	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, seriousAt.Add(2*time.Minute))
	if repeated := tracker.takeAlert(); repeated != nil {
		t.Fatalf("one serious phase should alert at most once: %+v", repeated)
	}
}

func TestThermalCriticalAlertsImmediately(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	status := tracker.observe(thermalReading{state: ThermalCritical, available: true}, thermalTestSnapshot(""), now)
	if status.State != ThermalCritical || !status.Active {
		t.Fatalf("critical state should be active immediately: %+v", status)
	}
	alert := tracker.takeAlert()
	if alert == nil || alert.State != ThermalCritical {
		t.Fatalf("critical state should notify immediately, got %+v", alert)
	}
}

func TestThermalSeriousTimerResetsAfterFair(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	start := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	snapshot := thermalTestSnapshot("")
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, start)
	tracker.observe(thermalReading{state: ThermalFair, available: true}, snapshot, start.Add(50*time.Second))
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, start.Add(51*time.Second))
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, start.Add(110*time.Second))
	if alert := tracker.takeAlert(); alert != nil {
		t.Fatalf("serious timer should restart after returning to fair: %+v", alert)
	}
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, start.Add(111*time.Second))
	if alert := tracker.takeAlert(); alert == nil {
		t.Fatal("serious should alert after a new continuous 60-second phase")
	}
}

func TestThermalNominalMustRemainStableBeforeEndingEpisode(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	start := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	snapshot := thermalTestSnapshot("")
	tracker.observe(thermalReading{state: ThermalFair, available: true}, snapshot, start)

	nominalAt := start.Add(5 * time.Minute)
	recovering := tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, nominalAt)
	if !recovering.Active || !recovering.Recovering {
		t.Fatalf("nominal should keep the episode recovering for two minutes: %+v", recovering)
	}
	stillRecovering := tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, nominalAt.Add(2*time.Minute-time.Second))
	if !stillRecovering.Active || !stillRecovering.Recovering {
		t.Fatalf("episode ended before the recovery delay elapsed: %+v", stillRecovering)
	}
	ended := tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, nominalAt.Add(2*time.Minute))
	if ended.Active || ended.Recovering || ended.State != ThermalNominal {
		t.Fatalf("episode should end after two stable nominal minutes: %+v", ended)
	}
}

func TestThermalSameCauseCooldownDoesNotNotifyAgain(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	tracker.lastNotified = now
	tracker.lastCauseKey = "agent-residual"
	snapshot := thermalTestSnapshot("agent-residual")
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, now.Add(time.Minute))
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, now.Add(2*time.Minute))
	if alert := tracker.takeAlert(); alert != nil {
		t.Fatalf("same cause inside the 15-minute cooldown must stay quiet: %+v", alert)
	}
}

func TestThermalDifferentCauseIsNotBlockedByOldCooldown(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	now := time.Date(2026, 11, 5, 10, 0, 0, 0, time.Local)
	tracker.lastNotified = now
	tracker.lastCauseKey = "old-cause"
	snapshot := thermalTestSnapshot("new-cause")
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, now.Add(time.Minute))
	tracker.observe(thermalReading{state: ThermalSerious, available: true}, snapshot, now.Add(2*time.Minute))
	alert := tracker.takeAlert()
	if alert == nil || alert.WorkloadID != "new-cause" {
		t.Fatalf("a different cause must be able to notify: %+v", alert)
	}
}

func TestThermalAttributionPrefersResidualMCPOverHealthyServer(t *testing.T) {
	snapshot := thermalTestSnapshot("agent-residual")
	attribution := attributeThermal(snapshot, ThermalSerious)
	if attribution.workloadID != "agent-residual" {
		t.Fatalf("residual MCP evidence should win attribution, got %+v", attribution)
	}
	if len(attribution.evidence) < 3 {
		t.Fatalf("attribution should expose supporting evidence: %+v", attribution.evidence)
	}
}

func TestThermalNotificationBodyRedactsSensitiveIdentifiers(t *testing.T) {
	attribution := thermalAttribution{
		workloadID:   "agent-sensitive",
		workloadName: "Codex · 会话 12345 /Users/alan/private --token sk-action-secret",
		cause:        "PID 12345 command --token sk-action-secret",
	}
	body := thermalAlertBody(attribution)
	for _, forbidden := range []string{"12345", "/Users", "sk-action-secret", "--token", "PID"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("thermal notification leaked %q: %s", forbidden, body)
		}
	}
	if !strings.Contains(body, "打开 AI 任务管理器") {
		t.Fatalf("notification should route the user to evidence: %s", body)
	}
}

func TestThermalHistoryDeduplicatesSameStateWithinOneMinute(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	start := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	snapshot := thermalTestSnapshot("")
	first := tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, start)
	tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, start.Add(25*time.Second))
	second := tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, start.Add(70*time.Second))

	if len(first.History) != 1 || len(second.History) != 2 {
		t.Fatalf("same state should sample once per minute: first=%+v second=%+v", first.History, second.History)
	}
	if second.History[1].ObservedAt != start.Add(70*time.Second).UnixMilli() {
		t.Fatalf("thermal sample must use the real observation timestamp: %+v", second.History[1])
	}
}

func TestThermalHistoryRecordsStateChangeImmediately(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	start := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	snapshot := thermalTestSnapshot("")
	tracker.observe(thermalReading{state: ThermalNominal, available: true}, snapshot, start)
	status := tracker.observe(thermalReading{state: ThermalFair, available: true}, snapshot, start.Add(5*time.Second))

	if len(status.History) != 2 || status.History[1].State != ThermalFair {
		t.Fatalf("state changes must be recorded immediately: %+v", status.History)
	}
}

func TestThermalHistoryRecordsUnavailableState(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	status := tracker.observe(thermalReading{state: ThermalUnavailable}, thermalTestSnapshot(""), now)
	if len(status.History) != 1 || status.History[0].State != ThermalUnavailable {
		t.Fatalf("unavailable observations must remain truthful history: %+v", status.History)
	}
}

func TestThermalHistoryKeepsBoundedRecentSamples(t *testing.T) {
	tracker := newThermalEpisodeTracker()
	start := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	snapshot := thermalTestSnapshot("")
	var status ThermalStatus
	for i := 0; i <= thermalHistoryLimit+5; i++ {
		state := ThermalNominal
		if i%2 == 1 {
			state = ThermalFair
		}
		status = tracker.observe(thermalReading{state: state, available: true}, snapshot, start.Add(time.Duration(i)*time.Minute))
	}
	if len(status.History) != thermalHistoryLimit {
		t.Fatalf("thermal history must stay bounded at %d, got %d", thermalHistoryLimit, len(status.History))
	}
	last := start.Add(time.Duration(thermalHistoryLimit+5) * time.Minute)
	if status.History[len(status.History)-1].ObservedAt != last.UnixMilli() {
		t.Fatalf("bounded history must retain the newest sample: %+v", status.History[len(status.History)-1])
	}
}
