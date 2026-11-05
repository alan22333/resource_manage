package monitor

import (
	"testing"
	"time"
)

func TestBuildDemoSnapshotHasConsistentEvidence(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 30, 0, 0, time.Local)
	snapshot := BuildDemoSnapshot(now, nil)

	if snapshot.Mode != "demo" || snapshot.GeneratedAt != now.UnixMilli() {
		t.Fatalf("unexpected snapshot identity: mode=%q generatedAt=%d", snapshot.Mode, snapshot.GeneratedAt)
	}
	if len(snapshot.Workloads) != 6 {
		t.Fatalf("want 6 demo workloads, got %d", len(snapshot.Workloads))
	}
	if snapshot.Summary.TopWorkloadID != "demo-codex-old-branch" {
		t.Fatalf("unexpected top workload: %q", snapshot.Summary.TopWorkloadID)
	}
	if snapshot.Summary.AttentionCount != 4 || snapshot.Summary.ReclaimCount != 3 {
		t.Fatalf("unexpected risk counts: attention=%d reclaim=%d", snapshot.Summary.AttentionCount, snapshot.Summary.ReclaimCount)
	}

	var totalRSS, reclaimRSS uint64
	var reclaimProcesses int
	for _, workload := range snapshot.Workloads {
		var processRSS uint64
		for _, proc := range workload.Processes {
			processRSS += proc.RSSBytes
		}
		if processRSS != workload.RSSBytes {
			t.Errorf("%s RSS mismatch: workload=%d process-sum=%d", workload.ID, workload.RSSBytes, processRSS)
		}
		totalRSS += workload.RSSBytes
		if workload.Reclaimable {
			for _, proc := range workload.Processes {
				if isReclaimTarget(workload, proc) {
					reclaimRSS += proc.RSSBytes
					reclaimProcesses++
				}
			}
		}
	}
	if totalRSS != snapshot.Summary.TotalRSSBytes {
		t.Fatalf("total RSS mismatch: summary=%d workload-sum=%d", snapshot.Summary.TotalRSSBytes, totalRSS)
	}
	if reclaimRSS != snapshot.Summary.ReclaimBytes || reclaimProcesses != snapshot.Summary.ReclaimProcessCount {
		t.Fatalf("reclaim accounting mismatch: summary=%d/%d expected=%d/%d",
			snapshot.Summary.ReclaimBytes, snapshot.Summary.ReclaimProcessCount, reclaimRSS, reclaimProcesses)
	}
	if snapshot.Summary.ReclaimBytes != 4_874*MB {
		t.Fatalf("want 4.874 GB reclaimable RSS, got %d", snapshot.Summary.ReclaimBytes)
	}

	for _, workload := range snapshot.Workloads {
		if workload.Kind != "agent" {
			continue
		}
		root := workload.Processes[0]
		if !root.Protected || root.ProtectedReason == "" {
			t.Errorf("%s agent workload root must be visibly protected: %+v", workload.ID, root)
		}
	}

	for _, id := range []string{"demo-codex-old-branch", "demo-claude-auth"} {
		var workload *Workload
		for index := range snapshot.Workloads {
			if snapshot.Workloads[index].ID == id {
				workload = &snapshot.Workloads[index]
				break
			}
		}
		if workload == nil {
			t.Fatalf("missing expected agent workload %s", id)
		}
		root := workload.Processes[0]
		if root.Kind != "agent" {
			t.Errorf("%s agent root must have agent kind: %+v", id, root)
		}
	}
}

func TestBuildDemoSnapshotAppliesStoppedWorkload(t *testing.T) {
	now := time.Date(2026, 11, 5, 10, 30, 0, 0, time.Local)
	stopped := map[string]ActionResult{
		"demo-codex-old-branch": {
			Token:          "receipt-token",
			WorkloadID:     "demo-codex-old-branch",
			WorkloadName:   "Codex · feature/old-branch",
			Message:        "演示：已安全停止 12 个进程",
			FinishedAt:     now.Add(-time.Minute).UnixMilli(),
			ReclaimedBytes: 3_048 * MB,
		},
	}
	snapshot := BuildDemoSnapshot(now, stopped)
	for _, workload := range snapshot.Workloads {
		if workload.ID == "demo-codex-old-branch" {
			t.Fatal("stopped workload still present")
		}
	}
	if len(snapshot.Events) == 0 || snapshot.Events[0].Kind != "reclaimed" {
		t.Fatalf("expected reclaim audit event first, got %#v", snapshot.Events)
	}
	if snapshot.Summary.ActiveWorkloads != 5 {
		t.Fatalf("want 5 active workloads, got %d", snapshot.Summary.ActiveWorkloads)
	}
}
