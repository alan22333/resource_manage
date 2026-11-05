package monitor

import (
	"testing"
	"time"
)

func TestClassifyProcessSignatures(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    string
	}{
		{name: "Code Helper", command: "Code Helper (Renderer)", want: "ide"},
		{name: "Codex", command: "codex --model gpt-5-codex", want: "agent"},
		{name: "Claude", command: "claude --resume auth-refactor", want: "agent"},
		{name: "MCP child", command: "node /tmp/.codex/mcp/server.js", want: "mcp"},
		{name: "Vite", command: "node_modules/.bin/vite --host 127.0.0.1", want: "server"},
		{name: "Terminal", command: "zsh -l", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyProcess("", tt.command); got != tt.want {
				t.Fatalf("classifyProcess(%q)=%q want %q", tt.command, got, tt.want)
			}
		})
	}
}

func TestFindAnchorPrefersAgentAncestorThenFallsBackToMCPGroup(t *testing.T) {
	ide := &rawProcess{pid: 100, ppid: 1, kind: "ide"}
	agent := &rawProcess{pid: 200, ppid: 100, kind: "agent"}
	mcp := &rawProcess{pid: 300, ppid: 200, kind: "mcp"}
	byPID := map[int32]*rawProcess{100: ide, 200: agent, 300: mcp}
	if got := findAnchor(300, byPID); got != agent {
		t.Fatalf("expected agent anchor, got %#v", got)
	}

	orphan := &rawProcess{pid: 500, ppid: 1, kind: "mcp"}
	if got := findAnchor(500, map[int32]*rawProcess{500: orphan}); got != orphan {
		t.Fatalf("expected orphan MCP fallback, got %#v", got)
	}
}

func TestBuildLiveWorkloadProtectsRootAndAccountsOnlyTargetRSS(t *testing.T) {
	now := time.Now()
	started := now.Add(-20 * time.Minute).UnixMilli()
	anchor := &rawProcess{
		pid: 1000, ppid: 1, name: "codex", command: "codex --model test",
		rss: 100 * MB, age: 1200, startedAt: started, kind: "agent",
	}
	members := []*rawProcess{anchor}
	for i := int32(1); i <= 4; i++ {
		members = append(members, &rawProcess{
			pid: 1000 + i, ppid: 1000, name: "node", command: "node mcp/server.js",
			rss: 200 * MB, age: 1200, startedAt: started + int64(i), kind: "mcp",
		})
	}
	byPID := map[int32]*rawProcess{}
	for _, member := range members {
		byPID[member.pid] = member
	}

	workload := buildLiveWorkload(anchor, members, byPID, now)
	if !workload.Reclaimable || workload.Risk != "reclaim" {
		t.Fatalf("expected reclaimable live workload, got risk=%s reclaimable=%v", workload.Risk, workload.Reclaimable)
	}
	if !workload.Processes[0].Protected {
		t.Fatal("agent root must be protected in live process tree")
	}
	snapshot := summarizeLive(now, []Workload{workload}, SystemMetrics{MemoryTotal: 16 * GB}, false)
	if snapshot.Summary.ReclaimBytes != 800*MB || snapshot.Summary.ReclaimProcessCount != 4 {
		t.Fatalf("summary counted protected root: %+v", snapshot.Summary)
	}
	if snapshot.Summary.TotalRSSBytes != 900*MB {
		t.Fatalf("unexpected task RSS total: %d", snapshot.Summary.TotalRSSBytes)
	}
}
