package monitor

import (
	"testing"
	"time"
)

func liveHistorySnapshot(now time.Time, risk string, cpu float64) Snapshot {
	return Snapshot{
		Mode: "live",
		Workloads: []Workload{{
			ID:         "agent-live",
			Name:       "Codex · live-test",
			Risk:       risk,
			RiskDetail: "本次采样的风险证据",
			CPUPercent: cpu,
			Trend:      []float64{cpu},
		}},
		GeneratedAt: now.UnixMilli(),
	}
}

func TestLiveHistoryUsesOnlyRealSamples(t *testing.T) {
	tracker := newLiveStateTracker()
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	first := liveHistorySnapshot(now, "healthy", 2.34)
	tracker.enrich(&first, now)

	if len(first.Workloads[0].Trend) != 1 || first.Workloads[0].Trend[0] != 2.3 {
		t.Fatalf("first sample must remain the only trend value: %+v", first.Workloads[0].Trend)
	}
	if len(first.Events) != 0 {
		t.Fatalf("healthy startup must not fabricate risk events: %+v", first.Events)
	}

	second := liveHistorySnapshot(now.Add(25*time.Second), "healthy", 7.86)
	tracker.enrich(&second, now.Add(25*time.Second))
	if len(second.Workloads[0].Trend) != 2 || second.Workloads[0].Trend[1] != 7.9 {
		t.Fatalf("second trend point must come from the second real sample: %+v", second.Workloads[0].Trend)
	}
}

func TestLiveHistoryRecordsRiskTransitionAtObservationTime(t *testing.T) {
	tracker := newLiveStateTracker()
	firstAt := time.Date(2026, 11, 8, 12, 0, 0, 0, time.Local)
	first := liveHistorySnapshot(firstAt, "attention", 62)
	tracker.enrich(&first, firstAt)

	transitionAt := firstAt.Add(25 * time.Second)
	second := liveHistorySnapshot(transitionAt, "healthy", 8)
	tracker.enrich(&second, transitionAt)
	if len(second.Events) != 2 {
		t.Fatalf("expected observed and recovery events, got %+v", second.Events)
	}
	if second.Events[0].Title != "任务信号已恢复稳定" || second.Events[0].Time != transitionAt.UnixMilli() {
		t.Fatalf("recovery event must carry real observation time: %+v", second.Events[0])
	}
	if second.Events[1].Time != firstAt.UnixMilli() {
		t.Fatalf("initial event must carry its original observation time: %+v", second.Events[1])
	}
}

func TestLiveHistoryRecordsActionReceipt(t *testing.T) {
	tracker := newLiveStateTracker()
	finishedAt := time.Date(2026, 11, 8, 12, 5, 0, 0, time.Local).UnixMilli()
	tracker.recordAction(ActionResult{
		WorkloadID:   "agent-live",
		WorkloadName: "Codex · live-test",
		Message:      "已发送 SIGTERM：2 个已退出",
		FinishedAt:   finishedAt,
	})

	snapshot := liveHistorySnapshot(time.Now(), "healthy", 1)
	tracker.enrich(&snapshot, time.Now())
	if len(snapshot.Events) != 1 {
		t.Fatalf("expected one action event, got %+v", snapshot.Events)
	}
	if snapshot.Events[0].Kind != "reclaimed" || snapshot.Events[0].Time != finishedAt {
		t.Fatalf("action event must be a truthful receipt: %+v", snapshot.Events[0])
	}
}
