//go:build unix

package runner

import (
	"fmt"
	"os/exec"
	"syscall"
)

func errnoMeansProcessGone(errno syscall.Errno) bool {
	fmt.Println("ERRNO", errno)
	switch errno {
	case syscall.ESRCH:
		return true
	default:
		return false
	}
}

func setPgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd, sig syscall.Signal) error {
	pid := cmd.Process.Pid
	if shouldUseProcessGroups {
		pid = -pid
	}
	return syscall.Kill(pid, sig)
}
