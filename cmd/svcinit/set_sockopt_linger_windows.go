//go:build windows

package main

import "syscall"

func setSockoptLinger(fd uintptr, level, opt int, l *syscall.Linger) error {
	return syscall.SetsockoptLinger(syscall.Handle(fd), level, opt, l)
}
