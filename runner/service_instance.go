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
	"time"

	"rules_itest/logger"
	"rules_itest/svclib"
)

type ServiceInstance struct {
	svclib.VersionedServiceSpec
	*exec.Cmd
	Stdin io.WriteCloser

	startTime     time.Time
	startDuration time.Duration

	startErrFn func() error
	waitErrFn  func() error

	mu     sync.Mutex
	runErr error
}

func (s *ServiceInstance) Start(_ context.Context) error {
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

	for {
		err := s.Error()
		if err != nil {
			return err
		}

		err = ctx.Err()
		if err != nil {
			return err
		}

		if s.HttpHealthCheckAddress != "" {
			log.Printf("HTTP Healthchecking %s (pid %d) : %s\n", coloredLabel, s.Process.Pid, s.HttpHealthCheckAddress)

			var resp *http.Response
			resp, err = http.DefaultClient.Get(s.HttpHealthCheckAddress)
			if resp != nil {
				if resp.StatusCode != http.StatusOK {
					err = fmt.Errorf("healthcheck for %s failed: %v", coloredLabel, resp)
				}

				closeErr := resp.Body.Close()
				if closeErr != nil {
					log.Printf("error closing http body %v", closeErr)
				}
			}

		} else if s.HealthCheck != "" {
			log.Printf("CMD Healthchecking %s (pid %d) : %s %v\n", coloredLabel, s.Process.Pid, s.Colorize(s.HealthCheckLabel), strings.Join(s.VersionedServiceSpec.HealthCheckArgs, " "))
			cmd := exec.CommandContext(ctx, s.HealthCheck, s.VersionedServiceSpec.HealthCheckArgs...)
			cmd.Stdout = logger.New(s.Label+"? ", s.Color, os.Stdout)
			cmd.Stderr = logger.New(s.Label+"? ", s.Color, os.Stderr)
			err = cmd.Run()
		}

		if err == nil {
			log.Printf("%s healthy!\n", coloredLabel)
			break
		}

		fmt.Println(err)
		time.Sleep(200 * time.Millisecond)
	}

	return nil
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
