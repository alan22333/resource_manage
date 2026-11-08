package monitor

import (
	"fmt"
	"sync"
	"time"
)

const liveTrendSamples = 36

type workloadLiveHistory struct {
	trend    []float64
	risk     string
	lastSeen time.Time
}

// liveStateTracker keeps short, process-local observation history. It never
// invents earlier values: a trend contains only CPU samples collected by this
// running app, and events carry the time at which a risk transition was seen.
type liveStateTracker struct {
	mu        sync.Mutex
	workloads map[string]*workloadLiveHistory
	events    []Event
}

func newLiveStateTracker() *liveStateTracker {
	return &liveStateTracker{workloads: make(map[string]*workloadLiveHistory)}
}

func (t *liveStateTracker) enrich(snapshot *Snapshot, now time.Time) {
	if snapshot == nil || snapshot.Mode != "live" {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	for i := range snapshot.Workloads {
		workload := &snapshot.Workloads[i]
		history := t.workloads[workload.ID]
		if history == nil {
			history = &workloadLiveHistory{}
			t.workloads[workload.ID] = history
		}

		history.trend = append(history.trend, round1(workload.CPUPercent))
		if len(history.trend) > liveTrendSamples {
			history.trend = append([]float64(nil), history.trend[len(history.trend)-liveTrendSamples:]...)
		}
		workload.Trend = append([]float64(nil), history.trend...)

		if history.risk == "" {
			if workload.Risk != "healthy" {
				t.addEventLocked(Event{
					ID:           fmt.Sprintf("live-observed-%s-%d", workload.ID, now.UnixNano()),
					Time:         now.UnixMilli(),
					Kind:         workload.Risk,
					WorkloadID:   workload.ID,
					WorkloadName: workload.Name,
					Title:        liveRiskTitle(workload.Risk, false),
					Detail:       workload.RiskDetail,
					Tone:         liveRiskTone(workload.Risk),
				})
			}
		} else if history.risk != workload.Risk {
			title := liveRiskTitle(workload.Risk, history.risk != "healthy" && workload.Risk == "healthy")
			t.addEventLocked(Event{
				ID:           fmt.Sprintf("live-transition-%s-%d", workload.ID, now.UnixNano()),
				Time:         now.UnixMilli(),
				Kind:         workload.Risk,
				WorkloadID:   workload.ID,
				WorkloadName: workload.Name,
				Title:        title,
				Detail:       workload.RiskDetail,
				Tone:         liveRiskTone(workload.Risk),
			})
		}

		history.risk = workload.Risk
		history.lastSeen = now
	}

	cutoff := now.Add(-2 * time.Hour)
	for id, history := range t.workloads {
		if !history.lastSeen.IsZero() && history.lastSeen.Before(cutoff) {
			delete(t.workloads, id)
		}
	}

	snapshot.Events = append([]Event(nil), t.events...)
}

func (t *liveStateTracker) recordAction(result ActionResult) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.addEventLocked(Event{
		ID:           fmt.Sprintf("live-action-%d", result.FinishedAt),
		Time:         result.FinishedAt,
		Kind:         "reclaimed",
		WorkloadID:   result.WorkloadID,
		WorkloadName: result.WorkloadName,
		Title:        "用户确认的安全回收已记录",
		Detail:       result.Message,
		Tone:         "healthy",
	})
}

func (t *liveStateTracker) addEventLocked(event Event) {
	t.events = append([]Event{event}, t.events...)
	if len(t.events) > 40 {
		t.events = t.events[:40]
	}
}

func liveRiskTitle(risk string, recovered bool) string {
	if recovered {
		return "任务信号已恢复稳定"
	}
	switch risk {
	case "reclaim":
		return "本次巡检发现疑似残留"
	case "attention":
		return "本次巡检发现资源偏高"
	case "healthy":
		return "任务组恢复稳定"
	default:
		return "本次巡检状态变化"
	}
}

func liveRiskTone(risk string) string {
	switch risk {
	case "reclaim":
		return "danger"
	case "attention":
		return "attention"
	default:
		return "healthy"
	}
}
