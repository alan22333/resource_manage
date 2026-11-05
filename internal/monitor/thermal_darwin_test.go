//go:build darwin

package monitor

import "testing"

func TestReadThermalStateUsesKnownPublicPressureLevel(t *testing.T) {
	reading := readThermalState()
	if !reading.available {
		t.Fatalf("public thermalState API should be available on supported macOS: %+v", reading)
	}
	switch reading.state {
	case ThermalNominal, ThermalFair, ThermalSerious, ThermalCritical:
		return
	default:
		t.Fatalf("unexpected thermal state: %+v", reading)
	}
}
