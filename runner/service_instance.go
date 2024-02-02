package runner

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"rules_itest/svclib"
)

type ServiceInstance struct {
	svclib.VersionedServiceSpec
	*exec.Cmd

	startTime     time.Time
	startDuration time.Duration

	startErrFn func() error

	mu     sync.Mutex
	runErr error
}

func (s *ServiceInstance) Start() error {
	s.startTime = time.Now()
	return s.startErrFn()
}

func (s *ServiceInstance) WaitUntilHealthy() error {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		s.startDuration = time.Since(s.startTime)
	}()

	if s.Type == "task" {
		return s.Wait()
	}

	for {
		err := s.Error()
		if err != nil {
			return err
		}

		log.Printf("Healthchecking %s\n", s.Label)

		if s.HttpHealthCheckAddress != "" {
			var resp *http.Response
			resp, err = http.DefaultClient.Get(s.HttpHealthCheckAddress)
			if resp != nil {
				defer resp.Body.Close()
			}

		} else if s.HealthCheck != "" {
			cmd := exec.Command(s.HealthCheck)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			err = cmd.Run()
		}

		if err == nil {
			log.Printf("%s healthy!\n", s.Label)
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
