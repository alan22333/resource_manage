package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aitaskmanager/ai-task-manager/internal/monitor"
	"github.com/egoist/mygo"
)

func TestThermalMonitorShowsAlertWithBoundedContext(t *testing.T) {
	alert := &monitor.ThermalAlert{State: monitor.ThermalSerious, Title: "系统热压力偏高", Body: "打开应用查看证据"}
	notified := 0
	monitor := &thermalMonitor{
		check: func() (monitor.ThermalStatus, *monitor.ThermalAlert, error) {
			return monitor.ThermalStatus{State: monitor.ThermalSerious}, alert, nil
		},
		notify: func(ctx context.Context, got monitor.ThermalAlert) error {
			notified++
			if got.Title != alert.Title {
				t.Fatalf("unexpected alert: %+v", got)
			}
			if _, ok := ctx.Deadline(); !ok {
				t.Fatal("notification delivery must have a finite deadline")
			}
			return nil
		},
		logf: t.Logf,
	}

	monitor.checkOnce(context.Background())
	if notified != 1 {
		t.Fatalf("expected one notification, got %d", notified)
	}
}

func TestThermalMonitorStopsAfterNotificationPermissionDenied(t *testing.T) {
	alert := &monitor.ThermalAlert{State: monitor.ThermalCritical, Title: "系统热压力严重"}
	notified := 0
	logged := 0
	monitor := &thermalMonitor{
		check: func() (monitor.ThermalStatus, *monitor.ThermalAlert, error) {
			return monitor.ThermalStatus{}, alert, nil
		},
		notify: func(context.Context, monitor.ThermalAlert) error {
			notified++
			return mygo.ErrNotificationsDenied
		},
		logf: func(string, ...any) { logged++ },
	}

	monitor.checkOnce(context.Background())
	monitor.checkOnce(context.Background())

	if notified != 1 {
		t.Fatalf("permission denial must stop retries, notified %d times", notified)
	}
	if !monitor.notificationsDenied {
		t.Fatal("expected notification denial to be remembered")
	}
	if logged != 1 {
		t.Fatalf("expected one privacy-safe log line, got %d", logged)
	}
}

func TestThermalMonitorDoesNotNotifyWithoutAlert(t *testing.T) {
	notified := false
	monitor := &thermalMonitor{
		check: func() (monitor.ThermalStatus, *monitor.ThermalAlert, error) {
			return monitor.ThermalStatus{State: monitor.ThermalNominal}, nil, nil
		},
		notify: func(context.Context, monitor.ThermalAlert) error {
			notified = true
			return nil
		},
		logf: t.Logf,
	}

	monitor.checkOnce(context.Background())
	if notified {
		t.Fatal("nominal thermal state must not notify")
	}
}

func TestThermalMonitorLogsCollectionFailureWithoutNotifying(t *testing.T) {
	notified := false
	logged := false
	monitor := &thermalMonitor{
		check: func() (monitor.ThermalStatus, *monitor.ThermalAlert, error) {
			return monitor.ThermalStatus{}, nil, errors.New("collector unavailable")
		},
		notify: func(context.Context, monitor.ThermalAlert) error {
			notified = true
			return nil
		},
		logf: func(string, ...any) { logged = true },
	}

	monitor.checkOnce(context.Background())
	if notified || !logged {
		t.Fatalf("collection failure should log without notifying: notified=%v logged=%v", notified, logged)
	}
}

func TestNotificationFocusWaitsForWindowReadiness(t *testing.T) {
	focus := &notificationFocus{}
	focus.handle("unrelated-notification")
	focus.handle(thermalNotificationID)

	activated := 0
	focus.bind(func() { activated++ })
	if activated != 1 {
		t.Fatalf("pending notification click should focus once the window binds, got %d", activated)
	}

	focus.handle(thermalNotificationID)
	if activated != 2 {
		t.Fatalf("subsequent clicks should focus the ready window, got %d", activated)
	}
}

func TestThermalMonitorRunStopsWithContext(t *testing.T) {
	checked := make(chan struct{}, 1)
	monitor := &thermalMonitor{
		interval: 5 * time.Millisecond,
		check: func() (monitor.ThermalStatus, *monitor.ThermalAlert, error) {
			select {
			case checked <- struct{}{}:
			default:
			}
			return monitor.ThermalStatus{State: monitor.ThermalNominal}, nil, nil
		},
		notify: func(context.Context, monitor.ThermalAlert) error { return nil },
		logf:   t.Logf,
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		monitor.run(ctx)
		close(done)
	}()

	select {
	case <-checked:
	case <-time.After(time.Second):
		t.Fatal("thermal monitor did not run")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("thermal monitor did not stop after cancellation")
	}
}
