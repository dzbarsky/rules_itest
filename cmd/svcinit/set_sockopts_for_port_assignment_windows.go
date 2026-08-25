//go:build windows

package main

import "syscall"

func setSockoptsForPortAssignment(fd uintptr, l *syscall.Linger) error {
	err := syscall.SetsockoptLinger(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, l)
	if err != nil {
		return err
	}

	// Windows has no SO_REUSEPORT. SO_REUSEADDR allows the service listener to
	// bind while svcinit holds a bind-only reservation socket.
	return syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
}
