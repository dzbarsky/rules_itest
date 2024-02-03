package runner

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"time"

	"rules_itest/logger"
	"rules_itest/svclib"
)

type ServiceSpecs = map[string]svclib.VersionedServiceSpec

type runner struct {
	serviceSpecs ServiceSpecs

	serviceInstances map[string]*ServiceInstance
}

func New(serviceSpecs ServiceSpecs) *runner {
	r := &runner{
		serviceInstances: map[string]*ServiceInstance{},
	}
	r.UpdateSpecs(serviceSpecs)
	return r
}

func (r *runner) StartAll() error {
	starter := newTopologicalStarter(r.serviceInstances)
	return starter.Run()
}

func (r *runner) StopAll() (map[string]*os.ProcessState, error) {
	states := make(map[string]*os.ProcessState)

	for _, serviceInstance := range r.serviceInstances {
		stopInstance(serviceInstance)
		states[serviceInstance.Label] = serviceInstance.Cmd.ProcessState
	}

	return states, nil
}

type updateActions struct {
	toStopLabels  []string
	toStartLabels []string
}

func computeUpdateActions(currentServices, newServices ServiceSpecs) updateActions {
	actions := updateActions{}

	// Check if existing services need a restart or a shutdown.
	for label, service := range currentServices {
		newService, ok := newServices[label]
		if !ok {
			fmt.Println(label + " has been removed, stopping")
			actions.toStopLabels = append(actions.toStopLabels, label)
			continue
		}

		// TODO(zbarsky): probably not needed in case service deps are changing
		if !reflect.DeepEqual(service, newService) {
			fmt.Println(label + " definition or code has changed, restarting...")
			actions.toStopLabels = append(actions.toStopLabels, label)
			actions.toStartLabels = append(actions.toStartLabels, label)
			continue
		}
	}

	// Handle new services
	for label := range newServices {
		if _, ok := currentServices[label]; !ok {
			actions.toStartLabels = append(actions.toStartLabels, label)
		}
	}

	return actions
}

func (r *runner) UpdateSpecs(serviceSpecs ServiceSpecs) {
	updateActions := computeUpdateActions(r.serviceSpecs, serviceSpecs)

	for _, label := range updateActions.toStopLabels {
		serviceInstance := r.serviceInstances[label]
		stopInstance(serviceInstance)
		delete(r.serviceInstances, label)
	}

	for _, label := range updateActions.toStartLabels {
		r.serviceInstances[label] = prepareServiceInstance(serviceSpecs[label])
	}
	r.serviceSpecs = serviceSpecs
}

func (r *runner) UpdateSpecsAndRestart(serviceSpecs ServiceSpecs) error {
	r.UpdateSpecs(serviceSpecs)
	return r.StartAll()
}

func prepareServiceInstance(s svclib.VersionedServiceSpec) *ServiceInstance {
	cmd := exec.Command(s.Exe, s.Args...)
	for k, v := range s.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Stdout = logger.New(s.Label+"> ", s.Color, os.Stdout)
	cmd.Stderr = logger.New(s.Label+"> ", s.Color, os.Stderr)

	return &ServiceInstance{
		VersionedServiceSpec: s,
		Cmd:                  cmd,

		startErrFn: sync.OnceValue(cmd.Start),
	}
}

func stopInstance(serviceInstance *ServiceInstance) {
	serviceInstance.Cmd.Process.Kill()
	serviceInstance.Cmd.Wait()

	for serviceInstance.Cmd.ProcessState == nil {
		time.Sleep(5 * time.Millisecond)
	}
}
