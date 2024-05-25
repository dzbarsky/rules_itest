//go:build windows

package runner

import "os/exec"

func setPgid(cmd *exec.Cmd) {
	panic("Pgid not implemented on windows!")
}

func killGroup(cmd *exec.Cmd) {
	// Windows doesn't have process groups, so just kill the process.
	cmd.Process.Kill()
}
