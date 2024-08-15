package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"rules_itest/logger"
	"rules_itest/svclib"
)

type ServiceInstance struct {
	svclib.VersionedServiceSpec
	stdin io.WriteCloser
	cmd   *exec.Cmd

	startTime     time.Time
	startDuration time.Duration

	startErrFn func() error
	waitErrFn  func() error

	mu     sync.Mutex
	runErr error
	killed bool
}

func (s *ServiceInstance) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// If the process has finished running, we need to reinitialize the cmd.
	if s.cmd.ProcessState != nil {
		initializeServiceCmd(ctx, s)
	}
	s.startTime = time.Now()
	return s.startErrFn()
}

func (s *ServiceInstance) WaitUntilHealthy(ctx context.Context) error {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.startDuration = time.Since(s.startTime)
	}()

	if s.Type == "group" {
		return nil
	}

	coloredLabel := s.Colorize(s.Label)
	if s.Type == "task" {
		err := s.waitErrFn()
		log.Printf("%s completed.\n", coloredLabel)
		return err
	}

	sleepDuration, err := time.ParseDuration(s.VersionedServiceSpec.HealthCheckInterval)
	if err != nil {
		log.Printf("failed to parse health check time duration, falling back to 200ms: %v", err)
		// This should really not happen if we validate it properly in starlark
		sleepDuration = time.Duration(200) * time.Millisecond
	}

	for {
		err := s.Error()
		if err != nil {
			return err
		}

		err = ctx.Err()
		if err != nil {
			return err
		}

		err = s.HealthCheck(ctx)
		if err == nil {
			log.Printf("%s healthy!\n", coloredLabel)
			break
		}

		fmt.Println(err)

		time.Sleep(sleepDuration)
	}

	return nil
}

var httpClient = http.Client{
	// It's important to have a reasonable timeout here since the connection may never get accepted
	// if it's to a port that is SO_REUSEPORT-aware. In that case, the healthcheck will hang forever
	// without this timeout.
	Timeout: 50 * time.Millisecond,
}

func (s *ServiceInstance) HealthCheck(ctx context.Context) error {
	httpHealthCheckReq, _ := http.NewRequestWithContext(ctx, "GET", s.HttpHealthCheckAddress, nil)
	coloredLabel := s.Colorize(s.Label)

	var err error
	if s.HttpHealthCheckAddress != "" {
		log.Printf("HTTP Healthchecking %s (pid %d) : %s\n", coloredLabel, s.Pid(), s.HttpHealthCheckAddress)

		var resp *http.Response
		resp, err = httpClient.Do(httpHealthCheckReq)
		if resp != nil {
			if resp.StatusCode != http.StatusOK {
				err = fmt.Errorf("healthcheck for %s failed: %v", coloredLabel, resp)
			}

			closeErr := resp.Body.Close()
			if closeErr != nil {
				log.Printf("error closing http body %v", closeErr)
			}
		}

	} else if s.ServiceSpec.HealthCheck != "" {
		log.Printf("CMD Healthchecking %s (pid %d) : %s %v\n", coloredLabel, s.Pid(), s.Colorize(s.HealthCheckLabel), strings.Join(s.HealthCheckArgs, " "))
		cmd := exec.CommandContext(ctx, s.ServiceSpec.HealthCheck, s.HealthCheckArgs...)
		cmd.Stdout = logger.New(s.Label+"? ", s.Color, os.Stdout)
		cmd.Stderr = logger.New(s.Label+"? ", s.Color, os.Stderr)
		err = cmd.Run()
	}

	return err
}

func (s *ServiceInstance) StartTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startTime
}

func (s *ServiceInstance) StartDuration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startDuration
}

func (s *ServiceInstance) Error() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runErr
}

func (s *ServiceInstance) Stop(sig syscall.Signal) error {
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.killed = true
	}()

	if s.cmd.Process == nil {
		return nil
	}

	err := killGroup(s.cmd, sig)
	if err != nil {
		return err
	}

	for s.cmd.ProcessState == nil {
		time.Sleep(5 * time.Millisecond)
	}
	return nil
}

func (s *ServiceInstance) Wait() error {
	return s.waitErrFn()
}

func (s *ServiceInstance) Pid() int {
	return s.cmd.Process.Pid
}

func (s *ServiceInstance) ProcessState() *os.ProcessState {
	return s.cmd.ProcessState
}

// Returns true if killed by svcinit / svcctl instead of exited normally or crashed.
func (s *ServiceInstance) Killed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.killed
}
