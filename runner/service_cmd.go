package runner

import (
	"os/exec"
	"sync"

	"rules_itest/svclib"
)

type ServiceCommand struct {
	svclib.Service
	*exec.Cmd

	mu      sync.Mutex
	runErr  error
	version []byte
}

func (s *ServiceCommand) Start() {
	go func() {
		err := s.Run()
		s.mu.Lock()
		defer s.mu.Unlock()
		s.runErr = err
	}()
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
