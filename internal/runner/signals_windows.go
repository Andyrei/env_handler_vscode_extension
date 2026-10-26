//go:build windows

package runner

import "os"

func forwardedSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

func exitCodeFor(state *os.ProcessState) int {
	if state == nil {
		return 1
	}
	code := state.ExitCode()
	if code < 0 {
		return 1
	}
	return code
}
