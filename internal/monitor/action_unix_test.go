//go:build !windows

package monitor

import (
	"os/exec"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

func TestExecuteLiveStopsProcessWithSIGTERM(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	defer func() {
		_ = cmd.Process.Kill()
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	}()

	proc, err := process.NewProcess(int32(cmd.Process.Pid))
	if err != nil {
		t.Fatalf("inspect child: %v", err)
	}
	started, err := proc.CreateTime()
	if err != nil {
		t.Fatalf("read child start time: %v", err)
	}

	const rss = 2 * MB
	result := executeLive(ActionPlan{
		WorkloadID: "live-test",
		Targets: []ActionTarget{{
			PID:         int32(cmd.Process.Pid),
			StartTimeMs: started,
			Name:        "sleep",
			RSSBytes:    rss,
		}},
	})
	if len(result.Failed) != 0 {
		t.Fatalf("graceful stop failed: %+v", result.Failed)
	}
	if len(result.Stopped) != 1 || result.Stopped[0] != int32(cmd.Process.Pid) {
		t.Fatalf("process was not confirmed stopped: %+v", result)
	}
	if result.ReclaimedBytes != rss {
		t.Fatalf("want reclaimed RSS %d, got %d", rss, result.ReclaimedBytes)
	}
}
