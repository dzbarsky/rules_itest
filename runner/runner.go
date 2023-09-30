package runner

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"rules_itest/svclib"
)

type runner struct {
	services map[string]svclib.Service

	serviceCmds map[string]*ServiceCommand
}

func New(services map[string]svclib.Service) *runner {
	return &runner{
		services:    services,
		serviceCmds: make(map[string]*ServiceCommand),
	}
}

func (r *runner) StartAll() error {
	for _, service := range r.services {
		serviceCmd, err := newServiceCmd(service)
		if err != nil {
			return err
		}

		if service.VersionFile != "" {
			version, err := os.ReadFile(service.VersionFile)
			if err != nil {
				return err
			}
			serviceCmd.SetVersion(version)
		}

		r.serviceCmds[service.Label] = serviceCmd
	}

	starter := newTopologicalStarter(r.serviceCmds)
	return starter.Run()
}

func (r *runner) StopAll() (map[string]*os.ProcessState, error) {
	states := make(map[string]*os.ProcessState)

	for _, serviceCmd := range r.serviceCmds {
		stopService(serviceCmd)
		states[serviceCmd.Label] = serviceCmd.Cmd.ProcessState
	}

	return states, nil
}

func (r *runner) UpdateDefinitions(services map[string]svclib.Service) error {
	for _, service := range services {
		serviceCmd, ok := r.serviceCmds[service.Label]
		if !ok {
			var err error
			serviceCmd, err = newServiceCmd(service)
			if err != nil {
				return err
			}
			r.serviceCmds[service.Label] = serviceCmd
		}

		if service.VersionFile != "" {
			version, err := os.ReadFile(service.VersionFile)
			if err != nil {
				return err
			}
			if string(version) != string(serviceCmd.Version()) {
				fmt.Println(service.Label + " is stale, restarting...")
				stopService(serviceCmd)
				serviceCmd, err = newServiceCmd(service)
				if err != nil {
					return err
				}
				r.serviceCmds[service.Label] = serviceCmd
			}
			serviceCmd.SetVersion(version)
		}
	}

	starter := newTopologicalStarter(r.serviceCmds)
	return starter.Run()
}

func newServiceCmd(service svclib.Service) (*ServiceCommand, error) {
	cmd := exec.Command(service.Exe, service.Args...)
	for k, v := range service.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return &ServiceCommand{
		Service: service,
		Cmd:     cmd,

		startErrFn: sync.OnceValue(cmd.Start),
	}, nil
}

func stopService(serviceCmd *ServiceCommand) {
	serviceCmd.Cmd.Process.Kill()
	serviceCmd.Cmd.Wait()

	for serviceCmd.Cmd.ProcessState == nil {
		time.Sleep(5 * time.Millisecond)
	}
}
