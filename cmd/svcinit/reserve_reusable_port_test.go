package main

import (
	"context"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestReusablePortReservation(t *testing.T) {
	reservation, port, err := reserveReusablePort(0)
	if err != nil {
		t.Fatal(err)
	}
	defer reservation.Close()

	unawareListener, err := net.Listen("tcp4", "127.0.0.1:"+port)
	if err == nil {
		unawareListener.Close()
		t.Fatal("listener without a reusable-port option unexpectedly claimed the reserved port")
	}

	lc := net.ListenConfig{
		Control: func(network, address string, conn syscall.RawConn) error {
			var setSockoptErr error
			err := conn.Control(func(fd uintptr) {
				setSockoptErr = setSockoptsForPortAssignment(fd, &syscall.Linger{
					Onoff:  1,
					Linger: 0,
				})
			})
			if err != nil {
				return err
			}
			return setSockoptErr
		},
	}
	listener, err := lc.Listen(context.Background(), "tcp4", "127.0.0.1:"+port)
	if err != nil {
		t.Fatalf("listen on reserved port: %v", err)
	}
	defer listener.Close()

	acceptErr := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			acceptErr <- err
			return
		}
		conn.Close()
		acceptErr <- nil
	}()

	conn, err := net.DialTimeout("tcp4", "127.0.0.1:"+port, time.Second)
	if err != nil {
		t.Fatalf("dial service listener: %v", err)
	}
	conn.Close()

	select {
	case err := <-acceptErr:
		if err != nil {
			t.Fatalf("accept from service listener: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("connection was not accepted by the service listener")
	}
}
