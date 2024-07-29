//go:build unix

package main

import (
	"syscall"

	"golang.org/x/sys/unix"
)

type Fd = int

func setSockoptsForPortAssignment(fd int) error {
	// It's unfortunate that we need `unix` here; SO_REUSEPORT is defined on linuxarm64 but not linux...
	return syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
}
