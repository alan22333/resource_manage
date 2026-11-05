// Command demo-fixture writes the deterministic browser fixture used by the
// Vite preview, frontend tests, and visual regression runs.
package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"time"

	"github.com/aitaskmanager/ai-task-manager/internal/monitor"
)

func main() {
	out := flag.String("out", "src/fixtures/demo-snapshot.json", "output path")
	flag.Parse()
	now := time.Date(2026, 11, 5, 10, 30, 0, 0, time.Local)
	snapshot := monitor.BuildDemoSnapshot(now, nil)
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o755); err != nil {
		panic(err)
	}
	if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
		panic(err)
	}
}
