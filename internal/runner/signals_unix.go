//go:build !windows

package runner

import (
	"os"
	"syscall"
)

func forwardedSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}

func exitCodeFor(state *os.ProcessState) int {
	if state == nil {
		return 1
	}
	if status, ok := state.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal())
	}
	code := state.ExitCode()
	if code < 0 {
		return 1
	}
	return code
}
