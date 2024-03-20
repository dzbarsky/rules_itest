package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"syscall"
	"time"

	"rules_itest/logger"
	"rules_itest/runner/topological"
	"rules_itest/svclib"
)

type ServiceSpecs = map[string]svclib.VersionedServiceSpec

type runner struct {
	ctx          context.Context
	serviceSpecs ServiceSpecs

	serviceInstances map[string]*ServiceInstance
}

func New(ctx context.Context, serviceSpecs ServiceSpecs) *runner {
	r := &runner{
		ctx:              ctx,
		serviceInstances: map[string]*ServiceInstance{},
	}
	r.UpdateSpecs(serviceSpecs)
	return r
}

func (r *runner) StartAll() ([]topological.Task, error) {
	starter := newTopologicalStarter(r.serviceInstances)
	err := starter.Run(r.ctx)
	return starter.CriticalPath(), err
}

func (r *runner) StopAll() (map[string]*os.ProcessState, error) {
	states := make(map[string]*os.ProcessState)

	for _, serviceInstance := range r.serviceInstances {
		stopInstance(serviceInstance)
		states[serviceInstance.Label] = serviceInstance.Cmd.ProcessState
	}

	return states, nil
}

func (r *runner) GetStartDurations() map[string]time.Duration {
	durations := make(map[string]time.Duration)

	for _, serviceInstance := range r.serviceInstances {
		durations[serviceInstance.Label] = serviceInstance.startDuration
	}

	return durations
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
		r.serviceInstances[label] = prepareServiceInstance(r.ctx, serviceSpecs[label])
	}
	r.serviceSpecs = serviceSpecs
}

func (r *runner) UpdateSpecsAndRestart(serviceSpecs ServiceSpecs) ([]topological.Task, error) {
	r.UpdateSpecs(serviceSpecs)
	return r.StartAll()
}

func prepareServiceInstance(ctx context.Context, s svclib.VersionedServiceSpec) *ServiceInstance {
	cmd := exec.CommandContext(ctx, s.Exe, s.Args...)
	// Note, this leaks the caller's env into the service, so it's not hermetic.
	// For `bazel test`, Bazel is already sanitizing the env, so it's fine.
	// For `bazel run`, there is no expectation of hermeticity, and it can be nice to use env to control behavior.
	cmd.Env = os.Environ()
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
	serviceInstance.Cmd.Process.Signal(syscall.SIGTERM)
	serviceInstance.Cmd.Wait()

	for serviceInstance.Cmd.ProcessState == nil {
		time.Sleep(5 * time.Millisecond)
	}
}
