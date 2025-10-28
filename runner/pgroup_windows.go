//go:build windows

package runner

import (
	"os/exec"
	"syscall"
)

func errnoMeansProcessGone(errno syscall.Errno) bool {
	switch errno {
	case syscall.EINVAL:
		return true
	default:
		return false
	}
}

func setPgid(cmd *exec.Cmd) {
	panic("Pgid not implemented on windows!")
}

func killGroup(cmd *exec.Cmd, _ syscall.Signal) error {
	// Windows doesn't have process groups, so just kill the process.
	return cmd.Process.Kill()
}
