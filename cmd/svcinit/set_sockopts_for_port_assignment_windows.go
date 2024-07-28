//go:build windows

package main

import "syscall"

func setSockoptsForPortAssignment(fd uintptr, l *syscall.Linger) error {
	// Windows (even WSL) does not seem to support SO_REUSEPORT
	return syscall.SetsockoptLinger(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, l)
}
