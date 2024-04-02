//go:build windows

package runner

import (
	"os"
	"os/exec"
)

func setPgid(cmd *exec.Cmd) {}

func killGroup(p *os.Process) error {
	return p.Kill()
}
