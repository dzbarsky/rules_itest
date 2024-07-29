//go:build windows

package main

import "syscall"

type Fd = syscall.Handle

func setSockoptsForPortAssignment(fd syscall.Handle) error {
	// Windows (even WSL) does not seem to support SO_REUSEPORT
	return nil
}
