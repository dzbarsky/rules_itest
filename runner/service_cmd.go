package runner

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"rules_itest/svclib"
)

type ServiceInstance struct {
	svclib.ServiceSpec
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

	var port string
	for {
		err := s.Error()
		if err != nil {
			return err
		}

		if s.AutodetectPort {
			output, err := exec.Command("lsof",
				// Network connections
				"-i",
				// AND
				"-a",
				// Owned by our pid
				"-p", strconv.Itoa(s.Process.Pid),
				"-F", "n").CombinedOutput()
			if err != nil {
				fmt.Println("No port yet; waiting...")
				time.Sleep(200 * time.Millisecond)
				continue
			}

			// Output looks like so:
			//
			// p24051
			// f3
			// nlocalhost:52263
			parts := strings.Split(string(output), ":")
			port = parts[1]
			// Trim trailing \n
			port = port[:len(port)-1]
		}

		log.Printf("Healthchecking %s\n", s.Label)

		if s.HttpHealthCheckAddress != "" {
			// It's OK to mutate our spec since we have made a copy of it.
			s.HttpHealthCheckAddress = strings.ReplaceAll(
				s.HttpHealthCheckAddress, "$PORT", port)

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

	// Note, this can cause collisions. So be careful!
	portFile := strings.ReplaceAll(s.Label, "/", "_")

	return os.WriteFile(
		filepath.Join(os.Getenv("TEST_TMPDIR"), portFile), []byte(port), 0600)
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
