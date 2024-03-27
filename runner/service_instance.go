package runner

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"rules_itest/logger"
	"rules_itest/svclib"
)

func colorize(s svclib.VersionedServiceSpec) string {
	return s.Color + s.Label + logger.Reset
}

type ServiceInstance struct {
	svclib.VersionedServiceSpec
	*exec.Cmd
	Stdin io.WriteCloser

	startTime     time.Time
	startDuration time.Duration

	startErrFn func() error

	mu     sync.Mutex
	runErr error
}

func (s *ServiceInstance) Start(ctx context.Context) error {
	s.startTime = time.Now()
	return s.startErrFn()
}

func (s *ServiceInstance) WaitUntilHealthy(ctx context.Context) error {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.startDuration = time.Since(s.startTime)
	}()

	if s.Type == "task" {
		err := s.Wait()
		log.Printf("%s completed.\n", colorize(s.VersionedServiceSpec))
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

		log.Printf("Healthchecking %s (pid %d)\n", colorize(s.VersionedServiceSpec), s.Process.Pid)

		if s.HttpHealthCheckAddress != "" {
			var resp *http.Response
			resp, err = http.DefaultClient.Get(s.HttpHealthCheckAddress)
			if resp != nil {
				defer resp.Body.Close()
			}

		} else if s.HealthCheck != "" {
			cmd := exec.CommandContext(ctx, s.HealthCheck)
			cmd.Stdout = logger.New(s.Label+"? ", s.Color, os.Stdout)
			cmd.Stderr = logger.New(s.Label+"? ", s.Color, os.Stderr)
			err = cmd.Run()
		}

		if err == nil {
			log.Printf("%s healthy!\n", colorize(s.VersionedServiceSpec))
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
