package monitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"
)

type pendingPlan struct {
	plan      ActionPlan
	demo      bool
	createdAt time.Time
}

type confirmedPlan struct {
	plan        ActionPlan
	demo        bool
	confirmedAt time.Time
	expiresAt   time.Time
}

type Service struct {
	collector       *LiveCollector
	liveMu          sync.Mutex
	liveState       *liveStateTracker
	thermalEpisodes *thermalEpisodeTracker
	mu              sync.Mutex
	pending         map[string]pendingPlan
	confirmed       map[string]confirmedPlan
	demoStopped     map[string]ActionResult
	aiMu            sync.RWMutex
	aiOverride      *aiProviderConfig
}

func NewService() *Service {
	return &Service{
		collector:       NewLiveCollector(),
		liveState:       newLiveStateTracker(),
		thermalEpisodes: newThermalEpisodeTracker(),
		pending:         make(map[string]pendingPlan),
		confirmed:       make(map[string]confirmedPlan),
		demoStopped:     make(map[string]ActionResult),
	}
}

func (s *Service) Snapshot(demo bool) (Snapshot, error) {
	if demo {
		now := time.Now()
		s.mu.Lock()
		stopped := make(map[string]ActionResult, len(s.demoStopped))
		for id, result := range s.demoStopped {
			stopped[id] = result
		}
		s.mu.Unlock()
		return BuildDemoSnapshot(now, stopped), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Millisecond)
	defer cancel()
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	snapshot, err := s.collector.CollectLive(ctx)
	if err != nil {
		return Snapshot{}, err
	}
	now := time.Now()
	s.liveState.enrich(&snapshot, now)
	snapshot.Thermal = s.thermalEpisodes.observe(readThermalState(), snapshot, now)
	return snapshot, nil
}

// CheckThermal collects one live snapshot for the background thermal monitor
// and returns at most one notification-worthy alert. It never creates an
// action token and never sends a signal.
func (s *Service) CheckThermal() (ThermalStatus, *ThermalAlert, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 1800*time.Millisecond)
	defer cancel()
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	snapshot, err := s.collector.CollectLive(ctx)
	if err != nil {
		return ThermalStatus{}, nil, err
	}
	now := time.Now()
	s.liveState.enrich(&snapshot, now)
	status := s.thermalEpisodes.observe(readThermalState(), snapshot, now)
	return status, s.thermalEpisodes.takeAlert(), nil
}

func (s *Service) Preview(workloadID string, demo bool) (ActionPlan, error) {
	snapshot, err := s.Snapshot(demo)
	if err != nil {
		return ActionPlan{}, err
	}
	var workload *Workload
	for i := range snapshot.Workloads {
		if snapshot.Workloads[i].ID == workloadID {
			workload = &snapshot.Workloads[i]
			break
		}
	}
	if workload == nil {
		return ActionPlan{}, errors.New("任务不存在或已经退出，请刷新后重试")
	}
	if !snapshot.Capabilities.CanStop {
		return ActionPlan{}, errors.New("当前平台仅支持只读监控，不会发送进程信号")
	}
	if !workload.Reclaimable {
		return ActionPlan{}, errors.New("该任务没有足够证据进入回收计划")
	}

	plan := ActionPlan{
		WorkloadID:     workload.ID,
		WorkloadName:   workload.Name,
		Mode:           snapshot.Mode,
		EstimatedCPU:   workload.CPUPercent,
		ContainerNames: append([]string(nil), workload.ContainerNames...),
	}
	for _, proc := range workload.Processes {
		if proc.Protected {
			plan.Protected = append(plan.Protected, ProtectedProcess{PID: proc.PID, Name: proc.Name, Reason: proc.ProtectedReason})
			continue
		}
		if proc.Kind == "agent" && workload.Kind == "agent" {
			plan.Protected = append(plan.Protected, ProtectedProcess{PID: proc.PID, Name: proc.Name, Reason: "Agent 会话根保留，供会话状态与审计核对"})
			continue
		}
		if isReclaimTarget(*workload, proc) {
			plan.Targets = append(plan.Targets, ActionTarget{PID: proc.PID, StartTimeMs: proc.StartTimeMs, Name: proc.Name, RSSBytes: proc.RSSBytes})
			plan.EstimatedRSS += proc.RSSBytes
		}
	}
	if len(plan.Targets) == 0 {
		return ActionPlan{}, errors.New("没有可安全停止的后代进程")
	}
	token, err := randomToken()
	if err != nil {
		return ActionPlan{}, fmt.Errorf("创建动作令牌失败: %w", err)
	}
	now := time.Now()
	plan.Token = token
	plan.CreatedAtUnix = now.Unix()
	plan.ExpiresAtUnix = now.Add(2 * time.Minute).Unix()
	plan.Warnings = []string{
		"只会发送 SIGTERM，不会自动升级为强杀",
		"执行前会再次校验每个 PID 与启动时间",
		"受保护的 IDE、Shell 和任务根不会进入停止列表",
	}

	s.mu.Lock()
	s.pending[token] = pendingPlan{plan: plan, demo: demo, createdAt: now}
	s.cleanupLocked(now)
	s.mu.Unlock()
	return plan, nil
}

// Confirm converts a preview into a short-lived, single-use execution token.
// The desktop UI calls this only after the user reviews the complete blast
// radius and explicitly checks the confirmation control.
func (s *Service) Confirm(previewToken string) (ActionConfirmation, error) {
	s.mu.Lock()
	pending, ok := s.pending[previewToken]
	if ok {
		delete(s.pending, previewToken)
	}
	s.cleanupLocked(time.Now())
	s.mu.Unlock()
	if !ok {
		return ActionConfirmation{}, errors.New("预览令牌无效、已使用或已过期，请重新检查影响范围")
	}
	if time.Since(pending.createdAt) > 2*time.Minute {
		return ActionConfirmation{}, errors.New("动作预览已过期，请重新检查影响范围")
	}

	token, err := randomToken()
	if err != nil {
		return ActionConfirmation{}, fmt.Errorf("创建确认令牌失败: %w", err)
	}
	now := time.Now()
	expiresAt := now.Add(2 * time.Minute)
	s.mu.Lock()
	s.confirmed[token] = confirmedPlan{plan: pending.plan, demo: pending.demo, confirmedAt: now, expiresAt: expiresAt}
	s.cleanupLocked(now)
	s.mu.Unlock()
	return ActionConfirmation{
		Token:           token,
		WorkloadID:      pending.plan.WorkloadID,
		WorkloadName:    pending.plan.WorkloadName,
		TargetCount:     len(pending.plan.Targets),
		EstimatedRSS:    pending.plan.EstimatedRSS,
		ConfirmedAtUnix: now.Unix(),
		ExpiresAtUnix:   expiresAt.Unix(),
	}, nil
}

// Execute accepts only a confirmation token. Preview tokens are deliberately
// rejected so that opening a blast-radius dialog can never send a signal.
func (s *Service) Execute(token string) (ActionResult, error) {
	s.mu.Lock()
	pending, ok := s.confirmed[token]
	if ok {
		delete(s.confirmed, token)
	}
	s.cleanupLocked(time.Now())
	s.mu.Unlock()
	if !ok {
		return ActionResult{}, errors.New("执行令牌无效、已使用或未经确认，请重新预览并确认")
	}
	if time.Now().After(pending.expiresAt) {
		return ActionResult{}, errors.New("确认已过期，请重新检查影响范围")
	}

	if pending.demo {
		result := ActionResult{
			Token:          token,
			WorkloadID:     pending.plan.WorkloadID,
			WorkloadName:   pending.plan.WorkloadName,
			Mode:           "demo",
			Stopped:        make([]int32, 0, len(pending.plan.Targets)),
			Message:        fmt.Sprintf("演示：已安全停止 %d 个进程，预计释放 %s", len(pending.plan.Targets), formatBytes(pending.plan.EstimatedRSS)),
			ReclaimedBytes: pending.plan.EstimatedRSS,
			ConfirmedAt:    pending.confirmedAt.UnixMilli(),
			FinishedAt:     time.Now().UnixMilli(),
		}
		for _, target := range pending.plan.Targets {
			result.Stopped = append(result.Stopped, target.PID)
		}
		s.mu.Lock()
		s.demoStopped[pending.plan.WorkloadID] = result
		s.mu.Unlock()
		return result, nil
	}
	result := executeLive(pending.plan)
	result.Token = token
	result.ConfirmedAt = pending.confirmedAt.UnixMilli()
	s.liveState.recordAction(result)
	return result, nil
}

func executeLive(plan ActionPlan) ActionResult {
	result := ActionResult{
		Token:        plan.Token,
		WorkloadID:   plan.WorkloadID,
		WorkloadName: plan.WorkloadName,
		Mode:         "live",
		FinishedAt:   time.Now().UnixMilli(),
	}
	signaled := make([]ActionTarget, 0, len(plan.Targets))
	for _, target := range plan.Targets {
		if target.PID <= 1 {
			result.Failed = append(result.Failed, ActionFailure{PID: target.PID, Name: target.Name, Reason: "拒绝停止系统进程"})
			continue
		}
		proc, err := process.NewProcess(target.PID)
		if err != nil {
			result.Failed = append(result.Failed, ActionFailure{PID: target.PID, Name: target.Name, Reason: "进程已不存在"})
			continue
		}
		started, err := proc.CreateTime()
		if err != nil || !sameStartTime(started, target.StartTimeMs) {
			result.Failed = append(result.Failed, ActionFailure{PID: target.PID, Name: target.Name, Reason: "PID 身份校验失败，可能已被复用"})
			continue
		}
		if err := terminateProcess(target.PID); err != nil {
			result.Failed = append(result.Failed, ActionFailure{PID: target.PID, Name: target.Name, Reason: err.Error()})
			continue
		}
		signaled = append(signaled, target)
	}

	deadline := time.Now().Add(1200 * time.Millisecond)
	for len(signaled) > 0 && time.Now().Before(deadline) {
		time.Sleep(80 * time.Millisecond)
		remaining := signaled[:0]
		for _, target := range signaled {
			proc, err := process.NewProcess(target.PID)
			if err != nil {
				result.Stopped = append(result.Stopped, target.PID)
				result.ReclaimedBytes += target.RSSBytes
				continue
			}
			running, _ := proc.IsRunning()
			if !running {
				result.Stopped = append(result.Stopped, target.PID)
				result.ReclaimedBytes += target.RSSBytes
				continue
			}
			remaining = append(remaining, target)
		}
		signaled = remaining
	}
	for _, target := range signaled {
		result.Pending = append(result.Pending, target.PID)
	}
	result.Message = fmt.Sprintf("已发送 SIGTERM：%d 个已退出，%d 个仍在退出，%d 个失败", len(result.Stopped), len(result.Pending), len(result.Failed))
	return result
}

func (s *Service) cleanupLocked(now time.Time) {
	for token, pending := range s.pending {
		if now.Sub(pending.createdAt) > 2*time.Minute {
			delete(s.pending, token)
		}
	}
	for token, pending := range s.confirmed {
		if now.After(pending.expiresAt) {
			delete(s.confirmed, token)
		}
	}
}

func sameStartTime(actual, expected int64) bool {
	if expected <= 0 || actual <= 0 {
		return false
	}
	diff := actual - expected
	if diff < 0 {
		diff = -diff
	}
	return diff <= 2000
}

func randomToken() (string, error) {
	bytes := make([]byte, 18)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// isReclaimTarget is the single source of truth shared by previews and summary
// accounting. Protected roots are never counted as reclaimable capacity.
func isReclaimTarget(workload Workload, proc ProcessNode) bool {
	if proc.Protected || proc.Kind == "agent" {
		return false
	}
	return proc.Kind == "mcp" || workload.Kind == "mcp"
}
