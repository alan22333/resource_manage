package main

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"github.com/aitaskmanager/ai-task-manager/internal/monitor"
	"github.com/egoist/mygo"
)

const (
	thermalNotificationID      = "thermal-pressure"
	thermalMonitorInterval     = 25 * time.Second
	thermalNotificationTimeout = 20 * time.Second
)

type thermalCheckFunc func() (monitor.ThermalStatus, *monitor.ThermalAlert, error)
type thermalNotifyFunc func(context.Context, monitor.ThermalAlert) error
type thermalLogFunc func(format string, args ...any)

type thermalMonitor struct {
	interval time.Duration
	check    thermalCheckFunc
	notify   thermalNotifyFunc
	logf     thermalLogFunc

	notificationsDenied bool
}

func newThermalMonitor(service *monitor.Service) *thermalMonitor {
	return &thermalMonitor{
		interval: thermalMonitorInterval,
		check:    service.CheckThermal,
		notify:   showThermalNotification,
		logf:     log.Printf,
	}
}

func (m *thermalMonitor) run(ctx context.Context) {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.checkOnce(ctx)
		}
	}
}

func (m *thermalMonitor) checkOnce(ctx context.Context) {
	_, alert, err := m.check()
	if err != nil {
		m.logf("thermal check failed: %v", err)
		return
	}
	if alert == nil || m.notificationsDenied {
		return
	}

	notifyCtx, cancel := context.WithTimeout(ctx, thermalNotificationTimeout)
	defer cancel()
	err = m.notify(notifyCtx, *alert)
	switch {
	case err == nil:
		return
	case errors.Is(err, mygo.ErrNotificationsDenied):
		// Respect the user's decision. Thermal episodes continue to be tracked,
		// but the app must not hammer the system authorization prompt.
		m.notificationsDenied = true
		m.logf("thermal notifications are disabled by the user")
	case errors.Is(err, context.Canceled) && ctx.Err() != nil:
		return
	default:
		m.logf("thermal notification failed: %v", err)
	}
}

func showThermalNotification(ctx context.Context, alert monitor.ThermalAlert) error {
	return mygo.NewNotification(mygo.NotificationOptions{
		ID:    thermalNotificationID,
		Title: alert.Title,
		Body:  alert.Body,
	}).ShowContext(ctx)
}

// notificationFocus remembers a notification click that arrives before the
// main window exists. This is common when a notification click launches the
// app from a previous run.
type notificationFocus struct {
	mu      sync.Mutex
	focus   func()
	pending bool
}

func (n *notificationFocus) handle(id string) {
	if id != thermalNotificationID {
		return
	}
	n.request()
}

func (n *notificationFocus) request() {
	n.mu.Lock()
	focus := n.focus
	if focus == nil {
		n.pending = true
		n.mu.Unlock()
		return
	}
	n.mu.Unlock()
	focus()
}

func (n *notificationFocus) bind(focus func()) {
	n.mu.Lock()
	n.focus = focus
	shouldFocus := n.pending && focus != nil
	n.pending = false
	n.mu.Unlock()
	if shouldFocus {
		focus()
	}
}
