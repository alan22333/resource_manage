package monitor

import (
	"fmt"
	"sort"
	"time"
)

const (
	GB = uint64(1000 * 1000 * 1000)
	MB = uint64(1000 * 1000)
)

// BuildDemoSnapshot returns a deterministic dataset used by the public demo,
// screenshots, and frontend tests. The data is clearly labelled as demo in
// every surface and never mixes with live process identities.
func BuildDemoSnapshot(now time.Time, stopped map[string]ActionResult) Snapshot {
	nowMs := now.UnixMilli()
	workloads := demoWorkloads(now)
	filtered := workloads[:0]
	for _, workload := range workloads {
		if _, wasStopped := stopped[workload.ID]; !wasStopped {
			filtered = append(filtered, workload)
		}
	}
	workloads = filtered

	var totalRSS uint64
	var reclaimRSS uint64
	var reclaimProcesses int
	var attention int
	var reclaim int
	var top Workload
	var agentSessions int
	for _, workload := range workloads {
		totalRSS += workload.RSSBytes
		if workload.Reclaimable {
			reclaim++
			for _, proc := range workload.Processes {
				if isReclaimTarget(workload, proc) {
					reclaimRSS += proc.RSSBytes
					reclaimProcesses++
				}
			}
		}
		if workload.Risk == "attention" || workload.Risk == "reclaim" {
			attention++
		}
		if workload.Kind == "agent" {
			agentSessions++
		}
		if top.ID == "" || workload.RSSBytes > top.RSSBytes {
			top = workload
		}
	}

	events := demoEvents(nowMs)
	for _, result := range stopped {
		events = append([]Event{{
			ID:           "audit-" + result.Token,
			Time:         result.FinishedAt,
			Kind:         "reclaimed",
			WorkloadID:   result.WorkloadID,
			WorkloadName: result.WorkloadName,
			Title:        "安全回收已记录",
			Detail:       result.Message,
			Tone:         "healthy",
		}}, events...)
	}
	sort.Slice(events, func(i, j int) bool { return events[i].Time > events[j].Time })

	revision := fmt.Sprintf("demo-%d", now.Unix()/2)
	pressure := "旧分支会话留下了 12 个 MCP 子进程"
	if top.ID == "" {
		pressure = "演示中的异常任务已全部回收"
	}
	_, oldBranchStopped := stopped["demo-codex-old-branch"]
	return Snapshot{
		GeneratedAt: nowMs,
		Mode:        "demo",
		Revision:    revision,
		Summary: Summary{
			PressureLabel:       "内存压力偏高",
			PressureDetail:      pressure,
			TopWorkloadID:       top.ID,
			TopWorkloadName:     top.Name,
			ReclaimBytes:        reclaimRSS,
			ReclaimProcessCount: reclaimProcesses,
			TotalRSSBytes:       totalRSS,
			ActiveWorkloads:     len(workloads),
			AttentionCount:      attention,
			ReclaimCount:        reclaim,
			Confidence:          94,
		},
		System: SystemMetrics{
			CPUPercent:     67.4,
			MemoryUsed:     13_180 * MB,
			MemoryTotal:    24 * GB,
			SwapUsed:       1_820 * MB,
			SwapTotal:      4 * GB,
			ProcessCount:   418,
			AgentSessions:  agentSessions,
			ContainerCount: 2,
		},
		Thermal:   demoThermalStatus(now, oldBranchStopped),
		Workloads: workloads,
		Events:    events,
		Capabilities: Capabilities{
			CanStop:       true,
			Docker:        true,
			Elevated:      false,
			Platform:      "demo",
			CollectionMS:  38,
			LiveSupported: true,
		},
	}
}

func demoWorkloads(now time.Time) []Workload {
	started := func(minutes int64) int64 { return now.Add(-time.Duration(minutes) * time.Minute).UnixMilli() }

	oldCodexNodes := []ProcessNode{{
		ID: "demo-61201", PID: 61201, PPID: 824, Name: "codex", Command: "codex --model gpt-5-codex", RSSBytes: 92 * MB, CPUPercent: 0.4,
		AgeSeconds: 11340, Depth: 0, Kind: "agent", Protected: true, ProtectedReason: "Agent 会话根，保留用于会话状态与审计核对", StartTimeMs: started(189),
	}}
	for i := 0; i < 12; i++ {
		oldCodexNodes = append(oldCodexNodes, ProcessNode{
			ID: fmt.Sprintf("demo-%d", 61202+i), PID: int32(61202 + i), PPID: 61201,
			Name: "node", Command: fmt.Sprintf("node /Users/alan/.codex/mcp/server.js --workspace mygo-%d", i),
			RSSBytes: uint64(232+i*4) * MB, CPUPercent: float64(i%4) * 0.35,
			AgeSeconds: 11280, Depth: 1, Kind: "mcp", StartTimeMs: started(188),
		})
	}

	claudeNodes := []ProcessNode{{
		ID: "demo-63740", PID: 63740, PPID: 824, Name: "claude", Command: "claude --resume auth-refactor", RSSBytes: 186 * MB, CPUPercent: 1.2,
		AgeSeconds: 8340, Depth: 0, Kind: "agent", Protected: true, ProtectedReason: "Agent 会话根，保留用于会话状态与审计核对", StartTimeMs: started(139),
	}}
	for i := 0; i < 8; i++ {
		claudeNodes = append(claudeNodes, ProcessNode{
			ID: fmt.Sprintf("demo-%d", 63741+i), PID: int32(63741 + i), PPID: 63740,
			Name: "node", Command: fmt.Sprintf("node /Users/alan/.claude/mcp/postgres.js --profile dev-%d", i),
			RSSBytes: uint64(96+i*12) * MB, CPUPercent: 0.2, AgeSeconds: 8290, Depth: 1, Kind: "mcp", StartTimeMs: started(138),
		})
	}

	checkoutNodes := []ProcessNode{{
		ID: "demo-44102", PID: 44102, PPID: 2120, Name: "Code Helper", Command: "Code Helper (Renderer)", RSSBytes: 282 * MB, CPUPercent: 2.6,
		AgeSeconds: 2760, Depth: 0, Kind: "ide", Protected: true, ProtectedReason: "当前 IDE 宿主，默认保护", StartTimeMs: started(46),
	}}
	for i := 0; i < 5; i++ {
		checkoutNodes = append(checkoutNodes, ProcessNode{
			ID: fmt.Sprintf("demo-%d", 44103+i), PID: int32(44103 + i), PPID: 44102,
			Name: "codex", Command: fmt.Sprintf("codex mcp serve --thread checkout-%d", i), RSSBytes: uint64(50+i*9) * MB, CPUPercent: float64(i+1) * 1.1,
			AgeSeconds: 2700, Depth: 1, Kind: "mcp", StartTimeMs: started(45),
		})
	}

	docsNodes := []ProcessNode{{
		ID: "demo-58300", PID: 58300, PPID: 1, Name: "node", Command: "node /Users/alan/.claude/mcp/filesystem.js", RSSBytes: 95 * MB, CPUPercent: 0.1,
		AgeSeconds: 21720, Depth: 0, Kind: "mcp", StartTimeMs: started(362),
	}}
	for i := 0; i < 6; i++ {
		docsNodes = append(docsNodes, ProcessNode{
			ID: fmt.Sprintf("demo-%d", 58301+i), PID: int32(58301 + i), PPID: 58300,
			Name: "node", Command: fmt.Sprintf("node /Users/alan/.claude/mcp/search-worker.js --shard %d", i), RSSBytes: uint64(82+i*9) * MB, CPUPercent: 0.1,
			AgeSeconds: 21600, Depth: 1, Kind: "mcp", StartTimeMs: started(360),
		})
	}

	viteNodes := []ProcessNode{
		{ID: "demo-17801", PID: 17801, PPID: 1402, Name: "bun", Command: "bun run dev", RSSBytes: 38 * MB, CPUPercent: 1.8, AgeSeconds: 1840, Depth: 0, Kind: "server", StartTimeMs: started(31)},
		{ID: "demo-17802", PID: 17802, PPID: 17801, Name: "vite", Command: "vite --host 127.0.0.1", RSSBytes: 144 * MB, CPUPercent: 3.4, AgeSeconds: 1838, Depth: 1, Kind: "server", Protected: true, ProtectedReason: "正在监听 5173，保持运行", StartTimeMs: started(31)},
		{ID: "demo-17803", PID: 17803, PPID: 17802, Name: "esbuild", Command: "esbuild --service", RSSBytes: 62 * MB, CPUPercent: 0.6, AgeSeconds: 1838, Depth: 2, Kind: "build", StartTimeMs: started(31)},
	}

	dockerNodes := []ProcessNode{
		{ID: "demo-docker-1", PID: 23001, PPID: 998, Name: "com.docker.backend", Command: "com.docker.backend", RSSBytes: 286 * MB, CPUPercent: 1.1, AgeSeconds: 19620, Depth: 0, Kind: "container", Protected: true, ProtectedReason: "Docker 运行时，不随任务停止", StartTimeMs: started(327)},
		{ID: "demo-docker-2", PID: 23044, PPID: 23001, Name: "postgres", Command: "postgres -D /var/lib/postgresql/data", RSSBytes: 132 * MB, CPUPercent: 0.7, AgeSeconds: 19600, Depth: 1, Kind: "container", StartTimeMs: started(326)},
	}

	return []Workload{
		{
			ID: "demo-codex-old-branch", Name: "Codex · feature/old-branch", Kind: "agent", Source: "Codex CLI",
			CPUPercent: 4.9, RSSBytes: 3140 * MB, ProcessCount: len(oldCodexNodes), AgeSeconds: 11340,
			Risk: "reclaim", RiskLabel: "疑似残留", RiskDetail: "会话已 47 分钟无活动，12 个 MCP 后代仍在运行",
			Confidence: 96, Reclaimable: true, StartedAt: started(189), LastActivity: "47 分钟前",
			Trend: []float64{1.2, 1.4, 1.5, 1.8, 1.8, 2.0, 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 2.7, 2.8, 2.9, 3.0, 3.1, 3.1, 3.1, 3.1, 3.1, 3.1, 3.1, 3.1},
			Evidence: []Evidence{
				{Label: "重复 MCP", Detail: "12 个后代使用相同 server.js 签名", Tone: "attention"},
				{Label: "长时间闲置", Detail: "最后活动信号来自 47 分钟前", Tone: "attention"},
				{Label: "宿主已结束", Detail: "根会话不再接收工具调用，子进程仍存活", Tone: "danger"},
			},
			Processes: oldCodexNodes,
		},
		{
			ID: "demo-claude-auth", Name: "Claude Code · auth-refactor", Kind: "agent", Source: "Claude Code",
			CPUPercent: 2.8, RSSBytes: 1290 * MB, ProcessCount: len(claudeNodes), AgeSeconds: 8340,
			Risk: "reclaim", RiskLabel: "疑似残留", RiskDetail: "8 个 Postgres MCP 实例重复启动，空闲 33 分钟",
			Confidence: 91, Reclaimable: true, StartedAt: started(139), LastActivity: "33 分钟前",
			Trend: []float64{0.8, 0.9, 1.0, 1.0, 1.0, 1.1, 1.1, 1.2, 1.2, 1.2, 1.3, 1.3, 1.3, 1.3, 1.4, 1.4, 1.4, 1.4, 1.4, 1.4, 1.4, 1.4, 1.4, 1.4},
			Evidence: []Evidence{
				{Label: "重复实例", Detail: "同一 MCP server 在 8 个后代中重复", Tone: "attention"},
				{Label: "低活动", Detail: "最近 10 分钟 CPU 平均低于 0.2%", Tone: "neutral"},
				{Label: "可安全分离", Detail: "停止子进程不会影响当前 IDE", Tone: "healthy"},
			},
			Processes: claudeNodes,
		},
		{
			ID: "demo-codex-checkout", Name: "Codex · checkout-flow", Kind: "agent", Source: "VS Code",
			CPUPercent: 8.2, RSSBytes: 622 * MB, ProcessCount: len(checkoutNodes), AgeSeconds: 2760,
			Risk: "attention", RiskLabel: "持续活跃", RiskDetail: "5 个 MCP 进程正在处理当前会话",
			Confidence: 88, StartedAt: started(46), LastActivity: "刚刚",
			Trend: []float64{0.2, 0.4, 0.8, 1.1, 1.3, 1.3, 1.0, 1.4, 1.8, 2.2, 2.1, 2.4, 2.0, 2.2, 2.7, 2.4, 2.3, 2.6, 2.8, 2.4, 2.2, 2.1, 2.0, 1.9},
			Evidence: []Evidence{
				{Label: "当前宿主", Detail: "Codex 子进程仍挂在活跃的 VS Code 窗口", Tone: "healthy"},
				{Label: "资源集中", Detail: "RSS 主要来自 5 个会话专属 MCP", Tone: "neutral"},
			},
			Processes: checkoutNodes,
		},
		{
			ID: "demo-claude-orphan", Name: "Claude MCP · filesystem", Kind: "mcp", Source: "Orphan process",
			CPUPercent: 0.7, RSSBytes: 722 * MB, ProcessCount: len(docsNodes), AgeSeconds: 21720,
			Risk: "reclaim", RiskLabel: "孤立进程组", RiskDetail: "父会话已退出，6 小时前启动的 6 个 worker 仍存活",
			Confidence: 98, Reclaimable: true, StartedAt: started(362), LastActivity: "6 小时前",
			Trend: []float64{0.3, 0.3, 0.4, 0.4, 0.5, 0.5, 0.5, 0.6, 0.6, 0.6, 0.6, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7, 0.7},
			Evidence: []Evidence{
				{Label: "父进程已退出", Detail: "根 MCP 当前由 launchd 接管", Tone: "danger"},
				{Label: "长期存活", Detail: "已持续 6 小时 2 分钟", Tone: "attention"},
				{Label: "重复 worker", Detail: "6 个同签名 worker 共享同一父进程", Tone: "attention"},
			},
			Processes: docsNodes,
		},
		{
			ID: "demo-vite", Name: "Vite · localhost:5173", Kind: "server", Source: "Terminal",
			CPUPercent: 5.8, RSSBytes: 244 * MB, ProcessCount: len(viteNodes), AgeSeconds: 1840,
			Risk: "healthy", RiskLabel: "运行中", RiskDetail: "开发服务器健康，端口与构建服务正常",
			Confidence: 99, Protected: true, ProtectedReason: "监听中的开发服务器默认保护",
			StartedAt: started(31), LastActivity: "正在响应",
			Trend:     []float64{0.8, 1.1, 0.9, 1.2, 1.4, 1.1, 1.3, 1.6, 1.8, 1.7, 2.0, 2.2, 1.9, 2.4, 2.1, 2.8, 2.4, 2.0, 1.9, 2.2, 2.5, 2.1, 1.8, 1.7},
			Evidence:  []Evidence{{Label: "端口", Detail: "正在监听 127.0.0.1:5173", Tone: "healthy"}},
			Processes: viteNodes,
		},
		{
			ID: "demo-postgres", Name: "docker compose · postgres", Kind: "container", Source: "Docker",
			CPUPercent: 1.8, RSSBytes: 418 * MB, ProcessCount: len(dockerNodes), AgeSeconds: 19620,
			Risk: "healthy", RiskLabel: "运行中", RiskDetail: "容器健康，近 15 分钟负载稳定",
			Confidence: 93, Protected: true, ProtectedReason: "Docker 容器操作需单独确认",
			StartedAt: started(327), LastActivity: "2 秒前",
			Trend:     []float64{1.1, 1.2, 1.0, 1.1, 1.2, 1.3, 1.2, 1.4, 1.5, 1.4, 1.3, 1.6, 1.5, 1.3, 1.4, 1.6, 1.7, 1.5, 1.4, 1.5, 1.6, 1.8, 1.7, 1.8},
			Evidence:  []Evidence{{Label: "容器", Detail: "postgres-1 正在接受连接", Tone: "healthy"}},
			Processes: dockerNodes, ContainerNames: []string{"mygo-postgres-1"},
		},
	}
}

func demoEvents(nowMs int64) []Event {
	return []Event{
		{ID: "evt-1", Time: nowMs - 2*60*1000, Kind: "growth", WorkloadID: "demo-codex-old-branch", WorkloadName: "Codex · feature/old-branch", Title: "子进程再次增长", Detail: "旧分支新增 2 个 MCP 后代，内存增加 380 MB", Tone: "attention"},
		{ID: "evt-2", Time: nowMs - 9*60*1000, Kind: "idle", WorkloadID: "demo-claude-auth", WorkloadName: "Claude Code · auth-refactor", Title: "进入低活动状态", Detail: "连续 9 分钟没有工具调用或 shell 事件", Tone: "neutral"},
		{ID: "evt-3", Time: nowMs - 21*60*1000, Kind: "orphan", WorkloadID: "demo-claude-orphan", WorkloadName: "Claude MCP · filesystem", Title: "父会话退出", Detail: "根 MCP 已由 launchd 接管，6 个 worker 继续运行", Tone: "danger"},
		{ID: "evt-4", Time: nowMs - 34*60*1000, Kind: "start", WorkloadID: "demo-codex-checkout", WorkloadName: "Codex · checkout-flow", Title: "新会话开始", Detail: "Codex 通过 VS Code 启动 5 个 MCP 工具", Tone: "healthy"},
		{ID: "evt-5", Time: nowMs - 51*60*1000, Kind: "port", WorkloadID: "demo-vite", WorkloadName: "Vite · localhost:5173", Title: "开发服务器就绪", Detail: "localhost:5173 开始响应", Tone: "healthy"},
	}
}
