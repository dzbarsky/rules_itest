package runner

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"reflect"
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
		serviceCmd := newServiceCmd(service)

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

type updateActions struct {
	toStopLabels  []string
	toStartLabels []string
}

func computeUpdateActions(
	currentServices map[string]svclib.Service,
	currentVersionProvider func(label string) []byte,
	newServices map[string]svclib.Service,
	newVersionProvider func(label string) ([]byte, error),
) (
	updateActions, error,
) {
	actions := updateActions{}

	// Check if existing services need a restart or a shutdown.
	for label, service := range currentServices {
		newService, ok := newServices[label]
		if !ok {
			fmt.Println(label + "has been reoved, stopping")
			actions.toStopLabels = append(actions.toStopLabels, label)
			continue
		}

		// TODO(zbarsky): probably not needed in case service deps are changing
		if !reflect.DeepEqual(service, newService) {
			fmt.Println(label + "definition has changed, restarting...")
			actions.toStopLabels = append(actions.toStopLabels, label)
			actions.toStartLabels = append(actions.toStartLabels, label)
			continue
		}

		if service.VersionFile != "" {
			currentVersion := currentVersionProvider(label)
			newVersion, err := newVersionProvider(label)
			if err != nil {
				return actions, err
			}

			if !bytes.Equal(currentVersion, newVersion) {
				fmt.Println(label + "code has changed, restarting...")
				actions.toStopLabels = append(actions.toStopLabels, label)
				actions.toStartLabels = append(actions.toStartLabels, label)
			}
		}
	}

	// Handle new services
	for label := range newServices {
		if _, ok := currentServices[label]; !ok {
			actions.toStartLabels = append(actions.toStartLabels, label)
		}
	}

	return actions, nil
}

func (r *runner) UpdateDefinitions(services map[string]svclib.Service) error {
	updateActions, err := computeUpdateActions(
		r.services,
		func(label string) []byte {
			return r.serviceCmds[label].Version()
		},
		services,
		func(label string) ([]byte, error) {
			return os.ReadFile(services[label].VersionFile)
		},
	)
	if err != nil {
		return err
	}

	for _, label := range updateActions.toStopLabels {
		serviceCmd := r.serviceCmds[label]
		stopService(serviceCmd)
		delete(r.serviceCmds, label)
	}

	for _, label := range updateActions.toStartLabels {
		r.serviceCmds[label] = newServiceCmd(services[label])
	}

	r.services = services
	starter := newTopologicalStarter(r.serviceCmds)
	return starter.Run()
}

func newServiceCmd(service svclib.Service) *ServiceCommand {
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
	}
}

func stopService(serviceCmd *ServiceCommand) {
	serviceCmd.Cmd.Process.Kill()
	serviceCmd.Cmd.Wait()

	for serviceCmd.Cmd.ProcessState == nil {
		time.Sleep(5 * time.Millisecond)
	}
}
