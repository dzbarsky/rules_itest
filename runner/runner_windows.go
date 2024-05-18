//go:build windows

package runner

import "os/exec"

func setPgid(cmd *exec.Cmd) {
	panic("Pgid not implemented on windows!")
}

