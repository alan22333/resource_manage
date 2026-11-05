//go:build darwin

package monitor

import (
	"errors"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var (
	thermalReaderOnce sync.Once
	thermalReader     thermalProcessInfoReader
)

type thermalProcessInfoReader struct {
	err         error
	processInfo objc.ID
	thermalSel  objc.SEL
}

// readThermalState uses the public NSProcessInfo thermalState API through the
// Objective-C runtime. It deliberately avoids cgo so the production MyGo build
// behaves the same as a normal Go test build.
func readThermalState() thermalReading {
	thermalReaderOnce.Do(func() {
		// Foundation must be loaded before its classes are visible to the
		// Objective-C runtime. Both framework paths are part of macOS.
		if _, err := purego.Dlopen(
			"/System/Library/Frameworks/Foundation.framework/Foundation",
			purego.RTLD_GLOBAL|purego.RTLD_NOW,
		); err != nil {
			thermalReader.err = err
			return
		}

		class := objc.GetClass("NSProcessInfo")
		if class == 0 {
			thermalReader.err = errors.New("NSProcessInfo is unavailable")
			return
		}

		processInfo := objc.Send[objc.ID](objc.ID(class), objc.RegisterName("processInfo"))
		if processInfo == 0 {
			thermalReader.err = errors.New("NSProcessInfo.processInfo returned nil")
			return
		}

		thermalReader.processInfo = processInfo
		thermalReader.thermalSel = objc.RegisterName("thermalState")
	})
	if thermalReader.err != nil {
		return thermalReading{state: ThermalUnavailable, available: false}
	}

	// NSProcessInfo.thermalState is an NSInteger. MyGo currently targets 64-bit
	// macOS, where NSInteger maps to int64.
	state := objc.Send[int64](thermalReader.processInfo, thermalReader.thermalSel)
	switch state {
	case 0:
		return thermalReading{state: ThermalNominal, available: true}
	case 1:
		return thermalReading{state: ThermalFair, available: true}
	case 2:
		return thermalReading{state: ThermalSerious, available: true}
	case 3:
		return thermalReading{state: ThermalCritical, available: true}
	default:
		return thermalReading{state: ThermalUnavailable, available: false}
	}
}
