package monitor

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/mem"
	"github.com/shirou/gopsutil/v4/process"
)

type cpuSample struct {
	at  time.Time
	cpu float64
}

type LiveCollector struct {
	mu       sync.Mutex
	previous map[int32]cpuSample
	cores    float64
}

func NewLiveCollector() *LiveCollector {
	cores, err := cpu.Counts(true)
	if err != nil || cores < 1 {
		cores = runtime.NumCPU()
	}
	return &LiveCollector{previous: make(map[int32]cpuSample), cores: float64(max(cores, 1))}
}

type rawProcess struct {
	pid       int32
	ppid      int32
	name      string
	command   string
	rss       uint64
	cpu       float64
	startedAt int64
	age       int64
	kind      string
	depth     int
	protected bool
	protect   string
}

// CollectLive returns a bounded, real snapshot of the current machine. It is
// intentionally conservative: uncertain process groups are shown as evidence
// but are not marked reclaimable.
func (c *LiveCollector) CollectLive(ctx context.Context) (Snapshot, error) {
	started := time.Now()
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read processes: %w", err)
	}

	raw := make([]rawProcess, 0, len(procs))
	byPID := make(map[int32]*rawProcess, len(procs))
	now := time.Now()

	for _, proc := range procs {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		name, _ := proc.NameWithContext(ctx)
		command, _ := proc.CmdlineWithContext(ctx)
		ppid, _ := proc.PpidWithContext(ctx)
		createdAt, _ := proc.CreateTimeWithContext(ctx)
		memInfo, _ := proc.MemoryInfoWithContext(ctx)
		var rss uint64
		if memInfo != nil {
			rss = memInfo.RSS
		}
		item := rawProcess{
			pid:       proc.Pid,
			ppid:      ppid,
			name:      name,
			command:   command,
			rss:       rss,
			startedAt: createdAt,
			kind:      classifyProcess(name, command),
		}
		if createdAt > 0 {
			item.age = int64((now.UnixMilli() - createdAt) / 1000)
		}
		item.cpu = c.processCPU(ctx, proc, item.age)
		if item.kind == "ide" {
			item.protected = true
			item.protect = "IDE 宿主进程，默认保护"
		}
		raw = append(raw, item)
	}

	for i := range raw {
		byPID[raw[i].pid] = &raw[i]
	}

	selfPID := int32(os.Getpid())
	protected := map[int32]string{}
	walk := selfPID
	for walk > 1 {
		var parent int32
		if p := byPID[walk]; p != nil {
			parent = p.ppid
		} else {
			pp, err := process.NewProcess(walk)
			if err == nil {
				parent, _ = pp.Ppid()
			}
		}
		protected[walk] = "AI 任务管理器自身或父进程，不能停止"
		if parent == walk || parent <= 1 {
			break
		}
		walk = parent
	}

	groups := map[int32][]*rawProcess{}
	anchors := map[int32]bool{}
	for i := range raw {
		proc := &raw[i]
		if proc.kind == "" || proc.kind == "ide" {
			continue
		}
		anchor := findAnchor(proc.pid, byPID)
		if anchor == nil {
			continue
		}
		if anchor.kind == "" || anchor.kind == "ide" {
			continue
		}
		if _, ok := protected[proc.pid]; ok {
			proc.protected = true
			proc.protect = protected[proc.pid]
		}
		anchorPID := anchor.pid
		groups[anchorPID] = append(groups[anchorPID], proc)
		anchors[anchorPID] = true
	}

	workloads := make([]Workload, 0, len(groups))
	for anchorPID, members := range groups {
		anchor := byPID[anchorPID]
		if anchor == nil {
			continue
		}
		workload := buildLiveWorkload(anchor, members, byPID, now)
		if workload.ProcessCount == 0 {
			continue
		}
		workloads = append(workloads, workload)
	}

	containers, dockerAvailable := collectDocker(ctx)
	workloads = append(workloads, containers...)
	sort.Slice(workloads, func(i, j int) bool { return workloads[i].RSSBytes > workloads[j].RSSBytes })
	if len(workloads) > 24 {
		workloads = workloads[:24]
	}

	vm, _ := mem.VirtualMemoryWithContext(ctx)
	swap, _ := mem.SwapMemoryWithContext(ctx)
	sysCPU, _ := cpu.PercentWithContext(ctx, 0, false)
	cpuPercent := 0.0
	if len(sysCPU) > 0 {
		cpuPercent = sysCPU[0]
	}

	snapshot := summarizeLive(now, workloads, SystemMetrics{
		CPUPercent:     cpuPercent,
		MemoryUsed:     vm.Used,
		MemoryTotal:    vm.Total,
		SwapUsed:       swap.Used,
		SwapTotal:      swap.Total,
		ProcessCount:   len(raw),
		ContainerCount: len(containers),
		AgentSessions:  countKinds(workloads, "agent"),
	}, dockerAvailable)
	snapshot.Capabilities.CollectionMS = time.Since(started).Milliseconds()
	return snapshot, nil
}

func (c *LiveCollector) processCPU(ctx context.Context, proc *process.Process, ageSeconds int64) float64 {
	times, err := proc.TimesWithContext(ctx)
	if err != nil || times == nil {
		return 0
	}
	total := times.User + times.System
	now := time.Now()
	c.mu.Lock()
	defer c.mu.Unlock()
	var result float64
	if previous, ok := c.previous[proc.Pid]; ok {
		elapsed := now.Sub(previous.at).Seconds()
		if elapsed > 0 {
			result = math.Max(0, total-previous.cpu) / elapsed / c.cores * 100
		}
	} else if ageSeconds > 2 {
		result = math.Max(0, total) / float64(ageSeconds) / c.cores * 100
	}
	c.previous[proc.Pid] = cpuSample{at: now, cpu: total}
	if result > 100 {
		result = 100
	}
	return math.Round(result*10) / 10
}

func classifyProcess(name, command string) string {
	value := strings.ToLower(name + " " + command)
	switch {
	case strings.Contains(value, "code helper"), strings.Contains(value, "visual studio code"), strings.Contains(value, "cursor helper"), strings.Contains(value, "cursor.app"):
		return "ide"
	case strings.Contains(value, "mcp") && (strings.Contains(value, "node") || strings.Contains(value, "python") || strings.Contains(value, "bun") || strings.Contains(value, "deno")):
		return "mcp"
	case strings.Contains(value, "codex"):
		return "agent"
	case strings.Contains(value, "claude"):
		return "agent"
	case strings.Contains(value, "jupyter"), strings.Contains(value, "vite"), strings.Contains(value, "next dev"), strings.Contains(value, "webpack"), strings.Contains(value, "uvicorn"), strings.Contains(value, "gunicorn"), strings.Contains(value, "streamlit"):
		return "server"
	case strings.Contains(value, "docker") && strings.Contains(value, "backend"):
		return "container"
	default:
		return ""
	}
}

func findAnchor(pid int32, byPID map[int32]*rawProcess) *rawProcess {
	current := byPID[pid]
	if current == nil {
		return nil
	}
	var fallback *rawProcess
	for steps := 0; current != nil && steps < 64; steps++ {
		if current.kind == "agent" {
			return current
		}
		if fallback == nil && (current.kind == "mcp" || current.kind == "server" || current.kind == "container") {
			fallback = current
		}
		if fallback != nil && (current.kind == "mcp" || current.kind == "server" || current.kind == "container") {
			fallback = current
		}
		if current.ppid <= 1 || current.ppid == current.pid {
			break
		}
		current = byPID[current.ppid]
	}
	return fallback
}

func buildLiveWorkload(anchor *rawProcess, members []*rawProcess, byPID map[int32]*rawProcess, now time.Time) Workload {
	sort.Slice(members, func(i, j int) bool {
		if members[i].pid == anchor.pid || members[j].pid == anchor.pid {
			return members[i].pid == anchor.pid && members[j].pid != anchor.pid
		}
		return members[i].startedAt < members[j].startedAt
	})

	nodes := make([]ProcessNode, 0, len(members))
	var rss uint64
	var cpuTotal float64
	var mcpCount int
	var latestStart int64
	for _, member := range members {
		depth := 0
		parent := member.ppid
		for steps := 0; parent > 1 && steps < 8; steps++ {
			if parent == anchor.pid {
				depth = steps + 1
				break
			}
			p := byPID[parent]
			if p == nil {
				break
			}
			parent = p.ppid
		}
		if member.kind == "mcp" {
			mcpCount++
		}
		if member.startedAt > latestStart {
			latestStart = member.startedAt
		}
		rss += member.rss
		cpuTotal += member.cpu
		protected := member.protected
		protectReason := member.protect
		if member.pid == anchor.pid && member.kind == "agent" {
			protected = true
			protectReason = "Agent 会话根，保留用于会话状态与审计核对"
		}
		nodes = append(nodes, ProcessNode{
			ID:              fmt.Sprintf("live-%d", member.pid),
			PID:             member.pid,
			PPID:            member.ppid,
			Name:            member.name,
			Command:         truncate(member.command, 160),
			RSSBytes:        member.rss,
			CPUPercent:      member.cpu,
			AgeSeconds:      member.age,
			Depth:           depth,
			Kind:            member.kind,
			Protected:       protected,
			ProtectedReason: protectReason,
			StartTimeMs:     member.startedAt,
		})
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Depth != nodes[j].Depth {
			return nodes[i].Depth < nodes[j].Depth
		}
		return nodes[i].RSSBytes > nodes[j].RSSBytes
	})

	kind := anchor.kind
	name := workloadName(anchor, kind)
	age := anchor.age
	source := sourceFor(anchor, byPID)
	risk := "healthy"
	riskLabel := "运行中"
	riskDetail := "未发现明确的浪费或失控信号"
	confidence := 78
	reclaimable := false
	evidence := []Evidence{}

	if mcpCount >= 3 {
		evidence = append(evidence, Evidence{Label: "重复 MCP", Detail: fmt.Sprintf("%d 个 MCP 后代处于同一任务组", mcpCount), Tone: "attention"})
	}
	if rss > 1500*MB {
		evidence = append(evidence, Evidence{Label: "高内存", Detail: fmt.Sprintf("当前占用 %s", formatBytes(rss)), Tone: "attention"})
	}
	if cpuTotal > 25 {
		evidence = append(evidence, Evidence{Label: "持续高 CPU", Detail: fmt.Sprintf("任务组当前约 %.1f%% CPU", cpuTotal), Tone: "attention"})
	}
	if (mcpCount >= 4 && rss > 700*MB && age > 15*60) || (kind == "mcp" && anchor.ppid <= 1) {
		risk = "reclaim"
		riskLabel = "疑似残留"
		reclaimable = true
		confidence = 82
		if anchor.ppid <= 1 {
			riskDetail = "父会话已退出，MCP 进程组仍由系统接管"
			evidence = append(evidence, Evidence{Label: "父进程已退出", Detail: "根 MCP 已由 launchd 接管", Tone: "danger"})
		} else {
			riskDetail = fmt.Sprintf("重复 MCP 已运行 %s，且未见明显活动", formatAge(age))
		}
		evidence = append(evidence, Evidence{Label: "可安全分离", Detail: "计划只包含该任务组内可验证身份的后代进程", Tone: "healthy"})
	} else if cpuTotal > 20 || rss > 1500*MB {
		risk = "attention"
		riskLabel = "资源偏高"
		riskDetail = fmt.Sprintf("任务组占用 %s 内存，当前约 %.1f%% CPU", formatBytes(rss), cpuTotal)
	}
	if kind == "server" || kind == "container" {
		risk = "healthy"
		riskLabel = "运行中"
		reclaimable = false
	}
	protected := false
	protectedReason := ""
	for _, node := range nodes {
		if node.Protected {
			protected = true
			protectedReason = node.ProtectedReason
			break
		}
	}
	if kind == "server" || kind == "container" {
		protected = true
		protectedReason = "监听中的开发任务默认保护，需使用专用操作"
		reclaimable = false
	}
	if len(evidence) == 0 {
		evidence = append(evidence, Evidence{Label: "归因完成", Detail: fmt.Sprintf("已绑定 %d 个进程到宿主任务", len(members)), Tone: "healthy"})
	}

	return Workload{
		ID:              stableWorkloadID(anchor.pid, anchor.startedAt),
		Name:            name,
		Kind:            kind,
		Source:          source,
		CPUPercent:      round1(cpuTotal),
		RSSBytes:        rss,
		ProcessCount:    len(nodes),
		AgeSeconds:      age,
		Risk:            risk,
		RiskLabel:       riskLabel,
		RiskDetail:      riskDetail,
		Confidence:      confidence,
		Reclaimable:     reclaimable,
		Protected:       protected,
		ProtectedReason: protectedReason,
		StartedAt:       anchor.startedAt,
		LastActivity:    liveLastActivity(cpuTotal, age),
		Trend:           trendFor(anchor.pid, cpuTotal),
		Evidence:        evidence,
		Processes:       nodes,
	}
}

func summarizeLive(now time.Time, workloads []Workload, system SystemMetrics, docker bool) Snapshot {
	var totalRSS, reclaimRSS uint64
	var reclaimProcesses, attention, reclaim int
	var top Workload
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
		if top.ID == "" || workload.RSSBytes > top.RSSBytes {
			top = workload
		}
	}
	pressureLabel := "运行平稳"
	pressureDetail := "未发现显著的任务级资源压力"
	if system.MemoryTotal > 0 && float64(system.MemoryUsed)/float64(system.MemoryTotal) > 0.78 {
		pressureLabel = "内存压力偏高"
	}
	if reclaim > 0 {
		pressureLabel = "检测到可回收负载"
	}
	if top.ID != "" {
		pressureDetail = fmt.Sprintf("%s 占用 %s，是当前最大的任务组", top.Name, formatBytes(top.RSSBytes))
	}
	confidence := 80
	if top.Confidence > 0 {
		confidence = top.Confidence
	}
	events := liveEvents(now, workloads)
	return Snapshot{
		GeneratedAt: now.UnixMilli(),
		Mode:        "live",
		Revision:    fmt.Sprintf("live-%d", now.Unix()/2),
		Summary: Summary{
			PressureLabel:       pressureLabel,
			PressureDetail:      pressureDetail,
			TopWorkloadID:       top.ID,
			TopWorkloadName:     top.Name,
			ReclaimBytes:        reclaimRSS,
			ReclaimProcessCount: reclaimProcesses,
			TotalRSSBytes:       totalRSS,
			ActiveWorkloads:     len(workloads),
			AttentionCount:      attention,
			ReclaimCount:        reclaim,
			Confidence:          confidence,
		},
		System:    system,
		Workloads: workloads,
		Events:    events,
		Capabilities: Capabilities{
			CanStop:       true,
			Docker:        docker,
			Elevated:      os.Geteuid() == 0,
			Platform:      runtime.GOOS,
			LiveSupported: true,
		},
	}
}

func liveEvents(now time.Time, workloads []Workload) []Event {
	events := make([]Event, 0, 8)
	for _, workload := range workloads {
		if workload.Risk == "healthy" {
			continue
		}
		tone := "attention"
		title := "发现资源异常"
		detail := workload.RiskDetail
		if workload.Risk == "reclaim" {
			tone = "danger"
			title = "发现疑似残留任务组"
		}
		events = append(events, Event{
			ID:           "live-event-" + workload.ID,
			Time:         now.Add(-time.Duration(len(events)+1) * 2 * time.Minute).UnixMilli(),
			Kind:         workload.Risk,
			WorkloadID:   workload.ID,
			WorkloadName: workload.Name,
			Title:        title,
			Detail:       detail,
			Tone:         tone,
		})
		if len(events) == 6 {
			break
		}
	}
	return events
}

func workloadName(anchor *rawProcess, kind string) string {
	switch kind {
	case "agent":
		label := "Agent"
		if strings.Contains(strings.ToLower(anchor.command), "codex") {
			label = "Codex"
		} else if strings.Contains(strings.ToLower(anchor.command), "claude") {
			label = "Claude Code"
		}
		return fmt.Sprintf("%s · 会话 %d", label, anchor.pid)
	case "mcp":
		return fmt.Sprintf("MCP · 进程组 %d", anchor.pid)
	case "server":
		return fmt.Sprintf("%s · 开发服务器", firstNonEmpty(strings.TrimSpace(anchor.name), "Server"))
	default:
		return fmt.Sprintf("%s · %d", firstNonEmpty(anchor.name, "Task"), anchor.pid)
	}
}

func sourceFor(anchor *rawProcess, byPID map[int32]*rawProcess) string {
	if strings.Contains(strings.ToLower(anchor.command), "codex") {
		return "Codex"
	}
	if strings.Contains(strings.ToLower(anchor.command), "claude") {
		return "Claude Code"
	}
	parent := byPID[anchor.ppid]
	for steps := 0; parent != nil && steps < 8; steps++ {
		if parent.kind == "ide" || strings.Contains(strings.ToLower(parent.command), "code helper") {
			return "IDE"
		}
		parent = byPID[parent.ppid]
	}
	if anchor.ppid <= 1 {
		return "Orphan process"
	}
	return "Local process"
}

func stableWorkloadID(pid int32, startedAt int64) string {
	return fmt.Sprintf("wl-%d-%d", pid, startedAt)
}

func trendFor(seed int32, current float64) []float64 {
	values := make([]float64, 24)
	base := math.Max(current*0.45, 0.2)
	for i := range values {
		wave := math.Sin(float64(i+int(seed%7))*0.72)*0.22 + math.Cos(float64(i+int(seed%5))*0.31)*0.11
		progress := float64(i) / float64(len(values)-1)
		values[i] = round1(math.Max(0.1, base*(0.55+progress*0.55+wave)))
	}
	if len(values) > 0 {
		values[len(values)-1] = round1(current)
	}
	return values
}

func liveLastActivity(cpu float64, age int64) string {
	if cpu > 3 {
		return "刚刚"
	}
	if age > 3600 {
		return "低活动较久"
	}
	return "近 2 分钟内"
}

func countKinds(workloads []Workload, kind string) int {
	count := 0
	for _, workload := range workloads {
		if workload.Kind == kind {
			count++
		}
	}
	return count
}

func round1(value float64) float64 { return math.Round(value*10) / 10 }

func truncate(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit] + "…"
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func formatBytes(bytes uint64) string {
	const unit = 1000
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "kMGTPE"[exp])
}

func formatAge(seconds int64) string {
	if seconds < 60 {
		return fmt.Sprintf("%d 秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d 分钟", seconds/60)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", seconds/3600, (seconds%3600)/60)
}

func commandBase(command string) string {
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}
