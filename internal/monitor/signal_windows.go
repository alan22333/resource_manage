//go:build windows

package monitor

import (
	"fmt"
	"os"
)

func terminateProcess(pid int32) error {
	proc, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	_ = proc.Kill()
	return fmt.Errorf("Windows 的终止语义与 SIGTERM 不同，MVP 默认拒绝执行")
}
