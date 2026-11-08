package monitor

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ThermalNominal     = "nominal"
	ThermalFair        = "fair"
	ThermalSerious     = "serious"
	ThermalCritical    = "critical"
	ThermalUnavailable = "unavailable"

	defaultSeriousDelay  = 60 * time.Second
	defaultRecoveryDelay = 2 * time.Minute
	defaultAlertCooldown = 15 * time.Minute

	thermalHistoryLimit       = 60
	thermalHistoryMinInterval = time.Minute
)

// ThermalSample is one observed public thermal-pressure state. It deliberately
// contains no Celsius field: no synthetic temperature is mixed into this view.
type ThermalSample struct {
	State           string `json:"state"`
	ObservedAt      int64  `json:"observedAt"`
	DurationSeconds int64  `json:"durationSeconds"`
}

// ThermalStatus is a user-facing thermal episode. macOS exposes pressure
// levels, not a portable temperature value, so this model deliberately avoids
// a Celsius field and uses evidence-backed workload attribution instead.
type ThermalStatus struct {
	State             string          `json:"state"`
	Available         bool            `json:"available"`
	Active            bool            `json:"active"`
	Recovering        bool            `json:"recovering"`
	StartedAt         int64           `json:"startedAt"`
	DurationSeconds   int64           `json:"durationSeconds"`
	Cause             string          `json:"cause"`
	CauseWorkloadID   string          `json:"causeWorkloadId"`
	CauseWorkloadName string          `json:"causeWorkloadName"`
	Evidence          []Evidence      `json:"evidence"`
	Recommendation    string          `json:"recommendation"`
	Confidence        int             `json:"confidence"`
	ObservedAt        int64           `json:"observedAt"`
	History           []ThermalSample `json:"history"`
}

// ThermalAlert is emitted once after a sustained serious episode, or
// immediately for a critical episode. It carries no process action.
type ThermalAlert struct {
	State        string
	Title        string
	Body         string
	WorkloadID   string
	WorkloadName string
}

type thermalReading struct {
	state     string
	available bool
}

type thermalAttribution struct {
	cause          string
	workloadID     string
	workloadName   string
	recommendation string
	evidence       []Evidence
	confidence     int
}

type thermalEpisodeTracker struct {
	mu sync.Mutex

	seriousDelay  time.Duration
	recoveryDelay time.Duration
	alertCooldown time.Duration

	episodeActive   bool
	episodeStart    time.Time
	stableSince     time.Time
	seriousSince    time.Time
	seriousAlerted  bool
	criticalAlerted bool
	lastNotified    time.Time
	lastCauseKey    string
	last            thermalAttribution
	history         []ThermalSample
	lastHistoryAt   time.Time

	pending *ThermalAlert
}

func newThermalEpisodeTracker() *thermalEpisodeTracker {
	return &thermalEpisodeTracker{
		seriousDelay:  defaultSeriousDelay,
		recoveryDelay: defaultRecoveryDelay,
		alertCooldown: defaultAlertCooldown,
	}
}

func (m *thermalEpisodeTracker) observe(reading thermalReading, snapshot Snapshot, now time.Time) ThermalStatus {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !reading.available {
		m.recordHistoryLocked(ThermalUnavailable, now, 0)
		return ThermalStatus{
			State:          ThermalUnavailable,
			Available:      false,
			Cause:          "当前平台没有公开的系统热压力接口",
			Recommendation: "继续使用 CPU、内存与任务证据判断后台负载，不要猜测具体温度。",
			ObservedAt:     now.UnixMilli(),
			History:        m.historyCopyLocked(),
		}
	}

	state := normalizeThermalState(reading.state)
	recovering := false
	switch state {
	case ThermalNominal:
		if m.episodeActive {
			if m.stableSince.IsZero() {
				m.stableSince = now
			}
			if now.Sub(m.stableSince) >= m.recoveryDelay {
				m.clearEpisode()
			} else {
				recovering = true
			}
		}
		m.seriousSince = time.Time{}
		m.seriousAlerted = false
		m.criticalAlerted = false
	case ThermalFair:
		m.stableSince = time.Time{}
		m.beginEpisode(now)
		m.seriousSince = time.Time{}
		m.seriousAlerted = false
		m.criticalAlerted = false
	case ThermalSerious:
		m.stableSince = time.Time{}
		m.beginEpisode(now)
		if m.seriousSince.IsZero() {
			m.seriousSince = now
		}
		m.criticalAlerted = false
	case ThermalCritical:
		m.stableSince = time.Time{}
		m.beginEpisode(now)
		m.criticalAlerted = false
	}

	status := ThermalStatus{
		State:      state,
		Available:  true,
		Active:     m.episodeActive,
		Recovering: recovering,
		ObservedAt: now.UnixMilli(),
		Evidence:   []Evidence{},
		Confidence: 70,
		DurationSeconds: func() int64 {
			if !m.episodeActive || m.episodeStart.IsZero() {
				return 0
			}
			return max(0, int64(now.Sub(m.episodeStart)/time.Second))
		}(),
	}
	m.recordHistoryLocked(state, now, status.DurationSeconds)
	status.History = m.historyCopyLocked()
	if m.episodeActive {
		status.StartedAt = m.episodeStart.UnixMilli()
	}

	if !m.episodeActive {
		status.Cause = "未检测到持续热压力"
		status.Recommendation = "无需处理；应用会在热压力持续升高时自动关联后台任务。"
		return status
	}

	attribution := attributeThermal(snapshot, state)
	if recovering {
		attribution.cause = "热压力已回落，正在确认是否稳定"
		attribution.recommendation = "保持观察 2 分钟；不要因为刚从 Serious 回落就立即执行大量任务。"
		if len(m.last.evidence) > 0 {
			attribution.evidence = append([]Evidence(nil), m.last.evidence...)
		}
	}
	m.last = attribution
	status.Cause = attribution.cause
	status.CauseWorkloadID = attribution.workloadID
	status.CauseWorkloadName = attribution.workloadName
	status.Recommendation = attribution.recommendation
	status.Evidence = append([]Evidence(nil), attribution.evidence...)
	status.Confidence = attribution.confidence

	if state != ThermalSerious && state != ThermalCritical {
		return status
	}

	causeKey := attribution.workloadID
	if causeKey == "" {
		causeKey = "system"
	}
	cooldownElapsed := m.lastNotified.IsZero() || now.Sub(m.lastNotified) >= m.alertCooldown
	sameCauseCoolingDown := causeKey == m.lastCauseKey && !cooldownElapsed

	shouldAlert := false
	if state == ThermalCritical {
		if !m.criticalAlerted {
			m.criticalAlerted = true
			shouldAlert = true
		}
	} else if !m.seriousAlerted && !m.seriousSince.IsZero() && now.Sub(m.seriousSince) >= m.seriousDelay {
		m.seriousAlerted = true
		shouldAlert = true
	}
	if !shouldAlert || sameCauseCoolingDown {
		return status
	}

	m.lastNotified = now
	m.lastCauseKey = causeKey
	m.pending = &ThermalAlert{
		State:        state,
		Title:        thermalAlertTitle(state),
		Body:         thermalAlertBody(attribution),
		WorkloadID:   attribution.workloadID,
		WorkloadName: attribution.workloadName,
	}
	return status
}

func (m *thermalEpisodeTracker) takeAlert() *ThermalAlert {
	m.mu.Lock()
	defer m.mu.Unlock()
	alert := m.pending
	m.pending = nil
	return alert
}

func (m *thermalEpisodeTracker) beginEpisode(now time.Time) {
	if m.episodeActive {
		return
	}
	m.episodeActive = true
	m.episodeStart = now
	m.stableSince = time.Time{}
	m.seriousSince = time.Time{}
	m.seriousAlerted = false
	m.criticalAlerted = false
	m.last = thermalAttribution{}
}

func (m *thermalEpisodeTracker) clearEpisode() {
	m.episodeActive = false
	m.episodeStart = time.Time{}
	m.stableSince = time.Time{}
	m.seriousSince = time.Time{}
	m.seriousAlerted = false
	m.criticalAlerted = false
	m.last = thermalAttribution{}
}

func (m *thermalEpisodeTracker) recordHistoryLocked(state string, now time.Time, duration int64) {
	sameState := len(m.history) > 0 && m.history[len(m.history)-1].State == state
	if sameState && !m.lastHistoryAt.IsZero() && now.Sub(m.lastHistoryAt) < thermalHistoryMinInterval {
		return
	}
	m.history = append(m.history, ThermalSample{
		State:           state,
		ObservedAt:      now.UnixMilli(),
		DurationSeconds: max(0, duration),
	})
	if len(m.history) > thermalHistoryLimit {
		m.history = append([]ThermalSample(nil), m.history[len(m.history)-thermalHistoryLimit:]...)
	}
	m.lastHistoryAt = now
}

func (m *thermalEpisodeTracker) historyCopyLocked() []ThermalSample {
	return append([]ThermalSample(nil), m.history...)
}

func normalizeThermalState(state string) string {
	switch state {
	case ThermalNominal, ThermalFair, ThermalSerious, ThermalCritical:
		return state
	default:
		return ThermalUnavailable
	}
}

func thermalAlertTitle(state string) string {
	if state == ThermalCritical {
		return "系统热压力严重"
	}
	return "系统热压力偏高"
}

func thermalAlertBody(attribution thermalAttribution) string {
	if label := safeThermalLabel(attribution.workloadName); label != "" {
		return fmt.Sprintf("“%s”与当前热压力关联度最高。打开 AI 任务管理器查看证据和回收预览。", label)
	}
	return "未找到可归因任务。打开 AI 任务管理器查看系统信号。"
}

var thermalSensitiveDigits = regexp.MustCompile(`\d{3,}`)
var thermalPathLike = regexp.MustCompile(`(?:/Users|/home|/var|/private)/\S+`)
var thermalSecretLike = regexp.MustCompile(`(?i)\b(?:sk-[a-z0-9_-]+|ghp_[a-z0-9_-]+|api[_-]?key[=:]\S+)\b`)

// safeThermalLabel keeps notification copy useful without carrying process
// identifiers, paths, commands, or action tokens into the system UI.
func safeThermalLabel(value string) string {
	if index := strings.Index(value, " --"); index >= 0 {
		value = value[:index]
	}
	value = thermalSecretLike.ReplaceAllString(value, "后台任务")
	value = thermalPathLike.ReplaceAllString(value, "后台任务")
	value = thermalSensitiveDigits.ReplaceAllString(value, "进程组")
	value = strings.TrimSpace(value)
	if len([]rune(value)) > 48 {
		value = string([]rune(value)[:48]) + "…"
	}
	return value
}

func attributeThermal(snapshot Snapshot, state string) thermalAttribution {
	type candidate struct {
		workload Workload
		score    float64
		mcp      int
	}
	candidates := make([]candidate, 0, len(snapshot.Workloads))
	for _, workload := range snapshot.Workloads {
		mcp := 0
		for _, proc := range workload.Processes {
			if proc.Kind == "mcp" {
				mcp++
			}
		}
		score := workload.CPUPercent*2 + float64(mcp)*3
		if workload.RSSBytes > 2*GB {
			score += 12
		}
		if workload.AgeSeconds > 30*60 {
			score += 5
		}
		switch workload.Risk {
		case "reclaim":
			score += 55
		case "attention":
			score += 18
		}
		if workload.Reclaimable {
			score += 30
		}
		switch workload.Kind {
		case "server":
			score -= 20
		case "container":
			score -= 10
		case "agent", "mcp":
			score += 8
		}
		candidates = append(candidates, candidate{workload: workload, score: score, mcp: mcp})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })

	evidence := []Evidence{{
		Label:  "热压力",
		Detail: fmt.Sprintf("%s · 整机 CPU %.1f%%", thermalStateLabel(state), snapshot.System.CPUPercent),
		Tone:   thermalTone(state),
	}}
	if snapshot.System.MemoryTotal > 0 {
		memoryPercent := float64(snapshot.System.MemoryUsed) / float64(snapshot.System.MemoryTotal) * 100
		evidence = append(evidence, Evidence{
			Label:  "内存",
			Detail: fmt.Sprintf("已用 %.1f%% · Swap %s", memoryPercent, formatBytes(snapshot.System.SwapUsed)),
			Tone:   thermalMemoryTone(memoryPercent),
		})
	}
	if snapshot.System.ContainerCount > 0 {
		evidence = append(evidence, Evidence{Label: "Docker", Detail: fmt.Sprintf("%d 个容器在线，未单独抬高归因分数", snapshot.System.ContainerCount), Tone: "neutral"})
	} else {
		evidence = append(evidence, Evidence{Label: "Docker", Detail: "未检测到运行中的容器或 Docker 不可用", Tone: "neutral"})
	}

	if len(candidates) == 0 || candidates[0].score <= 8 {
		return thermalAttribution{
			cause:          "系统负载偏高，但未发现可归因的 Agent、MCP 或容器任务",
			recommendation: "继续观察 2 分钟；若热压力持续，检查系统更新、后台索引与外接显示器。",
			evidence:       evidence,
			confidence:     65,
		}
	}

	top := candidates[0]
	workload := top.workload
	descendants := max(0, workload.ProcessCount-1)
	cause := ""
	switch {
	case workload.Reclaimable && top.mcp > 0:
		cause = fmt.Sprintf("“%s”仍在运行，%d 个 MCP 后代已有残留证据", workload.Name, top.mcp)
	case workload.CPUPercent >= 20:
		cause = fmt.Sprintf("“%s”持续消耗约 %.0f%% CPU", workload.Name, workload.CPUPercent)
	case top.mcp >= 3:
		cause = fmt.Sprintf("“%s”保有 %d 个 MCP 后代", workload.Name, top.mcp)
	case workload.RSSBytes > 2*GB:
		cause = fmt.Sprintf("“%s”占用 %s 内存", workload.Name, formatBytes(workload.RSSBytes))
	default:
		cause = fmt.Sprintf("“%s”是当前关联度最高的后台任务", workload.Name)
	}
	evidence = append(evidence, Evidence{
		Label:  "任务归因",
		Detail: fmt.Sprintf("%s · %d 个后代 · %.0f%% CPU", workload.Name, descendants, workload.CPUPercent),
		Tone:   "attention",
	})
	if workflowReason := thermalWorkflowEvidence(workload, top.mcp); workflowReason != "" {
		evidence = append(evidence, Evidence{Label: "排查线索", Detail: workflowReason, Tone: "attention"})
	}
	recommendation := fmt.Sprintf("展开“%s”的进程树核对持续高 CPU 的子进程；不要直接结束未知进程。", workload.Name)
	confidence := min(95, max(70, workload.Confidence-3))
	if workload.Reclaimable {
		recommendation = fmt.Sprintf("先预览“%s”的回收影响，核对受保护根与可分离后代；确认前不会发送信号。", workload.Name)
	}
	return thermalAttribution{
		cause:          cause,
		workloadID:     workload.ID,
		workloadName:   workload.Name,
		recommendation: recommendation,
		evidence:       evidence,
		confidence:     confidence,
	}
}

func thermalWorkflowEvidence(workload Workload, mcp int) string {
	if mcp > 0 && workload.AgeSeconds > 30*60 {
		return fmt.Sprintf("%d 个 MCP 后代已存在 %s", mcp, formatAge(workload.AgeSeconds))
	}
	if workload.ProcessCount > 1 {
		return fmt.Sprintf("%d 个后代仍与任务根保持父子关系", workload.ProcessCount-1)
	}
	return ""
}

func thermalStateLabel(state string) string {
	switch state {
	case ThermalCritical:
		return "Critical"
	case ThermalSerious:
		return "Serious"
	case ThermalFair:
		return "Fair"
	case ThermalNominal:
		return "Nominal"
	default:
		return "Unavailable"
	}
}

func thermalTone(state string) string {
	switch state {
	case ThermalCritical, ThermalSerious:
		return "danger"
	case ThermalFair:
		return "attention"
	default:
		return "healthy"
	}
}

func thermalMemoryTone(percent float64) string {
	switch {
	case percent >= 85:
		return "danger"
	case percent >= 72:
		return "attention"
	default:
		return "healthy"
	}
}

func demoThermalStatus(now time.Time, recovered bool) ThermalStatus {
	if recovered {
		return ThermalStatus{
			State:          ThermalNominal,
			Available:      true,
			Cause:          "演示中的热压力已随残留后代回收而回落",
			Recommendation: "无需处理；继续观察后台任务。",
			Evidence:       []Evidence{{Label: "热压力", Detail: "Nominal · 系统回到稳定区间", Tone: "healthy"}},
			Confidence:     92,
			ObservedAt:     now.UnixMilli(),
		}
	}
	return ThermalStatus{
		State:             ThermalSerious,
		Available:         true,
		Active:            true,
		StartedAt:         now.Add(-6*time.Minute - 12*time.Second).UnixMilli(),
		DurationSeconds:   372,
		Cause:             "“Codex · feature/old-branch”留下了 12 个 MCP 后代",
		CauseWorkloadID:   "demo-codex-old-branch",
		CauseWorkloadName: "Codex · feature/old-branch",
		Evidence: []Evidence{
			{Label: "热压力", Detail: "Serious · 整机 CPU 67.4%", Tone: "danger"},
			{Label: "任务归因", Detail: "旧分支仍有 12 个 MCP 后代，占用 3.0 GB", Tone: "attention"},
			{Label: "内存", Detail: "已用 54.9% · Swap 1.8 GB", Tone: "healthy"},
			{Label: "Docker", Detail: "2 个容器在线，负载稳定", Tone: "healthy"},
		},
		Recommendation: "先预览旧分支的回收影响，核对受保护 Agent 根与 12 个可分离后代；确认前不会发送信号。",
		Confidence:     91,
		ObservedAt:     now.UnixMilli(),
	}
}

func thermalCauseSummary(status ThermalStatus) string {
	if strings.TrimSpace(status.Cause) == "" {
		return "正在收集热事件证据"
	}
	return status.Cause
}
