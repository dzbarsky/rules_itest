//go:build unix

package main

import "syscall"

func setSockoptLinger(fd uintptr, level, opt int, l *syscall.Linger) error {
	return syscall.SetsockoptLinger(int(fd), level, opt, l)
}
