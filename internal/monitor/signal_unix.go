//go:build !windows

package monitor

import "syscall"

func terminateProcess(pid int32) error {
	return syscall.Kill(int(pid), syscall.SIGTERM)
}
