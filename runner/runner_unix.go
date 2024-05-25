//go:build unix

package runner

import (
	"os/exec"
	"syscall"
)

func setPgid(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killGroup(cmd *exec.Cmd) {
	pid := cmd.Process.Pid
	if shouldUseProcessGroups {
		pid = -pid
	}
	syscall.Kill(pid, syscall.SIGKILL)
}
