//go:build windows

package monitor

import "errors"

// Process termination is intentionally disabled until the app can offer a
// verified Windows-specific graceful close path. This function must never call
// Kill and then report failure: doing so would send a signal despite the
// advertised safety boundary.
func terminateProcess(pid int32) error {
	return errors.New("Windows 版本仅支持只读监控；不会向进程发送终止信号")
}
