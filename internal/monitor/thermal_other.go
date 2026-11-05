//go:build !darwin

package monitor

func readThermalState() thermalReading {
	return thermalReading{state: ThermalUnavailable, available: false}
}
