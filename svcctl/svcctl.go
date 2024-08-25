//go:build go1.22

package svcctl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"rules_itest/runner"
	"syscall"
	"time"
)

type handlerFn = func(context.Context, *runner.Runner, chan error, http.ResponseWriter, *http.Request)

func handle(ctx context.Context, mux *http.ServeMux, r *runner.Runner, servicesErrCh chan error, pattern string, handler handlerFn) {
	mux.HandleFunc(pattern, func(w http.ResponseWriter, req *http.Request) {
		handler(ctx, r, servicesErrCh, w, req)
	})
}

func getService(r *runner.Runner, req *http.Request) (*runner.ServiceInstance, int, error) {
	params := req.URL.Query()
	service := params.Get("service")
	if service == "" {
		return nil, http.StatusBadRequest, fmt.Errorf("service parameter is required")
	}

	s := r.GetInstance(service)
	if s == nil {
		return nil, http.StatusBadRequest, fmt.Errorf("instance %q not found", service)
	}

	if s.Type != "service" {
		return nil, http.StatusBadRequest, fmt.Errorf("instance %q is not a service", service)
	}

	return s, http.StatusOK, nil
}

func handleHealthCheck(ctx context.Context, r *runner.Runner, _ chan error, w http.ResponseWriter, req *http.Request) {
	s, status, err := getService(r, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	err = s.HealthCheck(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleStart(ctx context.Context, r *runner.Runner, serviceErrCh chan error, w http.ResponseWriter, req *http.Request) {
	s, status, err := getService(r, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	err = s.Start(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// NOTE: it is important to wait here because we started the service without using `StartAll`,
	// which waits for processes to prevent them from turning into zombies.
	go func() {
		err := s.Wait()
		if err != nil && !s.Killed() {
			serviceErrCh <- fmt.Errorf(s.Colorize(s.Label) + " exited with error: " + err.Error())
		}
	}()

	w.WriteHeader(http.StatusOK)
}

func handleKill(ctx context.Context, r *runner.Runner, _ chan error, w http.ResponseWriter, req *http.Request) {
	sig := syscall.SIGKILL
	params := req.URL.Query()
	signal := params.Get("signal")
	if signal != "" {
		// Currently only SIGTERM and SIGKILL are supported.
		switch signal {
		case "SIGTERM":
			sig = syscall.SIGTERM
		case "SIGKILL":
			sig = syscall.SIGKILL
		default:
			http.Error(w, "unsupported signal", http.StatusBadRequest)
		}
	}

	s, status, err := getService(r, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	err = s.Stop(sig)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func handleWait(ctx context.Context, r *runner.Runner, _ chan error, w http.ResponseWriter, req *http.Request) {
	s, status, err := getService(r, req)
	if err != nil {
		http.Error(w, err.Error(), status)
		return
	}

	params := req.URL.Query()
	timeout := params.Get("timeout")
	var t time.Duration
	if timeout != "" {
		t, err = time.ParseDuration(timeout)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, t)
		ctx = timeoutCtx
		defer cancel()
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- s.Wait()
	}()

	select {
	case <-ctx.Done():
		http.Error(w, "timeout", http.StatusRequestTimeout)
	case err := <-errChan:
		if err == nil {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("0"))
			return
		}
		if err, ok := err.(*exec.ExitError); ok {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(fmt.Sprintf("%d", err.ExitCode())))
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func Serve(ctx context.Context, listener net.Listener, r *runner.Runner, servicesErrCh chan error) error {
	mux := http.NewServeMux()
	handle(ctx, mux, r, servicesErrCh, "GET /v0/healthcheck", handleHealthCheck)
	handle(ctx, mux, r, servicesErrCh, "GET /v0/start", handleStart)
	handle(ctx, mux, r, servicesErrCh, "GET /v0/kill", handleKill)
	handle(ctx, mux, r, servicesErrCh, "GET /v0/wait", handleWait)
	return http.Serve(listener, mux)
}
