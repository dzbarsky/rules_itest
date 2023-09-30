package runner

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"rules_itest/svclib"
)

type ServiceCommand struct {
	svclib.Service
	*exec.Cmd

	startTime     time.Time
	startDuration time.Duration

	startErrFn func() error

	mu      sync.Mutex
	runErr  error
	version []byte
}

func (s *ServiceCommand) Start() error {
	s.startTime = time.Now()
	return s.startErrFn()
	/*go func() {
		err := s.Run()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.runErr = err
	}()*/
}

func (s *ServiceCommand) WaitUntilHealthy() error {
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

		//log.Printf("Healthchecking %s at %s...\n", service.Label, service.HttpHealthCheckAddress)
		resp, err := http.DefaultClient.Get(s.HttpHealthCheckAddress)
		if resp != nil {
			defer resp.Body.Close()
		}
		if err == nil {
			log.Printf("%s healthy!\n", s.Label)
			return nil
		}

		fmt.Println(err)
		time.Sleep(200 * time.Millisecond)
	}
}

func (s *ServiceCommand) StartTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startTime
}

func (s *ServiceCommand) StartDuration() time.Duration {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startDuration
}

func (s *ServiceCommand) Error() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runErr
}

func (s *ServiceCommand) Version() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.version
}

func (s *ServiceCommand) SetVersion(version []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version = version
}
