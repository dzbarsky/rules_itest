package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"time"

	"rules_itest/logger"
	"rules_itest/runner/topological"
	"rules_itest/svclib"
)

// We need to use process groups to reliably tear down services and their descendants.
// This is especially important in hot-reload mode, where you need to restart the child
// and have it bind the same port.
// However, we don't want to do this in tests, because Bazel will already terminate the
// test process (svcinit) and all its children.
// If we were to start new process groups in tests, we could leak children (at least on Mac).
var shouldUseProcessGroups = runtime.GOOS != "windows" && os.Getenv("BAZEL_TEST") != "1"

type ServiceSpecs = map[string]svclib.VersionedServiceSpec

type Runner struct {
	ctx          context.Context
	serviceSpecs ServiceSpecs

	serviceInstances map[string]*ServiceInstance
}

func New(ctx context.Context, serviceSpecs ServiceSpecs) (*Runner, error) {
	r := &Runner{
		ctx:              ctx,
		serviceInstances: map[string]*ServiceInstance{},
	}
	err := r.UpdateSpecs(serviceSpecs, nil)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func colorize(s svclib.VersionedServiceSpec) string {
	return s.Colorize(s.Label)
}

func (r *Runner) StartAll(serviceErrCh chan error) ([]topological.Task, error) {
	tasks := allTasks(r.serviceInstances, func(ctx context.Context, service *ServiceInstance) error {
		if service.Type == "group" {
			return nil
		}
		log.Printf("Starting %s %v\n", colorize(service.VersionedServiceSpec), service.cmd.Args[1:])
		startErr := service.Start(ctx)
		if startErr != nil {
			return startErr
		}
		if service.VersionedServiceSpec.HealthCheckTimeout != "" {
			timeout, err := time.ParseDuration(service.VersionedServiceSpec.HealthCheckTimeout)
			if err != nil {
				log.Printf("failed to parse health check timeout, falling back to no timeout: %v", err)
			}
			timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
			ctx = timeoutCtx
			defer cancel()
		}
		return service.WaitUntilHealthy(ctx)
	})
	starter := topological.NewRunner(tasks)
	err := starter.Run(r.ctx)

	for _, service := range r.serviceInstances {
		if service.Type != "service" {
			continue
		}

		// TODO(zbarsky): Can remove the loop var once Go is sufficiently upgraded.
		go func(service *ServiceInstance) {
			err := service.Wait()
			if err != nil && !service.Killed() {
				serviceErrCh <- fmt.Errorf(colorize(service.VersionedServiceSpec) + " exited with error: " + err.Error())
			}
		}(service)
	}

	return starter.CriticalPath(), err
}

func (r *Runner) StopAll() (map[string]syscall.Rusage, error) {
	tasks := allTasks(r.serviceInstances, func(ctx context.Context, service *ServiceInstance) error {
		if service.Type == "group" {
			return nil
		}
		log.Printf("Stopping %s\n", colorize(service.VersionedServiceSpec))
		// Rusage wrapper reinterprets SIGUSR2 as SIGKILL for the underlying binary.
		service.Stop(syscall.SIGUSR2)
		return nil
	})
	stopper := topological.NewReversedRunner(tasks)
	err := stopper.Run(r.ctx)

	rusages := make(map[string]syscall.Rusage)

	for _, serviceInstance := range r.serviceInstances {
		if serviceInstance.Type == "group" {
			continue
		}
		data, err := os.ReadFile(serviceInstance.RusageFile)
		if err != nil {
			log.Print("Could not read rusage for", serviceInstance.Label)
			continue
		}

		var rusage syscall.Rusage
		err = json.Unmarshal(data, &rusage)
		if err != nil {
			return nil, err
		}
		rusages[serviceInstance.Label] = rusage
	}

	return rusages, err
}

func (r *Runner) GetStartDurations() map[string]time.Duration {
	durations := make(map[string]time.Duration)

	for _, serviceInstance := range r.serviceInstances {
		durations[serviceInstance.Label] = serviceInstance.startDuration
	}

	return durations
}

func (r *Runner) GetInstance(label string) *ServiceInstance {
	return r.serviceInstances[label]
}

type updateActions struct {
	toStopLabels   []string
	toStartLabels  []string
	toReloadLabels []string
}

func computeUpdateActions(currentServices, newServices ServiceSpecs) updateActions {
	actions := updateActions{}

	// Check if existing services need a reload, a restart, or a shutdown.
	for label, service := range currentServices {
		newService, ok := newServices[label]
		if !ok {
			fmt.Println(label + " has been removed, stopping")
			actions.toStopLabels = append(actions.toStopLabels, label)
			continue
		}

		// We technically don't need a restart if the change is the list of deps.
		// But that should not be a common use case, so it's not worth the complexity.
		if !reflect.DeepEqual(service, newService) {
			log.Printf(colorize(service) + " definition or code has changed, restarting...")
			if service.HotReloadable && reflect.DeepEqual(service.ServiceSpec, newService.ServiceSpec) {
				// The only difference is the Version. Trust the service that
				// it prefers to receive the ibazel reload command.
				actions.toReloadLabels = append(actions.toReloadLabels, label)
			} else {
				actions.toStopLabels = append(actions.toStopLabels, label)
				actions.toStartLabels = append(actions.toStartLabels, label)
			}
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

func (r *Runner) UpdateSpecs(serviceSpecs ServiceSpecs, ibazelCmd []byte) error {
	updateActions := computeUpdateActions(r.serviceSpecs, serviceSpecs)

	for _, label := range updateActions.toStopLabels {
		serviceInstance := r.serviceInstances[label]
		if serviceInstance.Type == "group" {
			continue
		}
		serviceInstance.Stop(syscall.SIGKILL)
		delete(r.serviceInstances, label)
	}

	for _, label := range updateActions.toStartLabels {
		var err error
		r.serviceInstances[label], err = prepareServiceInstance(r.ctx, serviceSpecs[label])
		if err != nil {
			return err
		}
	}

	for _, label := range updateActions.toReloadLabels {
		_, err := r.serviceInstances[label].stdin.Write(ibazelCmd)
		if err != nil {
			return err
		}
	}

	r.serviceSpecs = serviceSpecs
	return nil
}

func (r *Runner) UpdateSpecsAndRestart(
	serviceSpecs ServiceSpecs,
	serviceErrCh chan error,
	ibazelCmd []byte,
) (
	[]topological.Task, error,
) {
	err := r.UpdateSpecs(serviceSpecs, ibazelCmd)
	if err != nil {
		return nil, err
	}
	return r.StartAll(serviceErrCh)
}

func prepareServiceInstance(ctx context.Context, s svclib.VersionedServiceSpec) (*ServiceInstance, error) {
	if s.Type == "group" {
		return &ServiceInstance{
			VersionedServiceSpec: s,
			startErrFn:           sync.OnceValue(func() error { return nil }),
		}, nil
	}

	instance := &ServiceInstance{
		VersionedServiceSpec: s,
	}

	err := initializeServiceCmd(ctx, instance)
	if err != nil {
		return nil, err
	}

	return instance, nil
}

func initializeServiceCmd(ctx context.Context, instance *ServiceInstance) error {
	s := instance.VersionedServiceSpec

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

	if shouldUseProcessGroups {
		setPgid(cmd)
	}

	// Even if a child process exits, Wait will block until the I/O pipes are closed.
	// They may have been forwarded to an orphaned child, so we disable that behavior to unblock exit.
	if s.Type == "service" {
		cmd.WaitDelay = 1
	}

	instance.cmd = cmd
	instance.killed = false
	instance.startErrFn = sync.OnceValue(cmd.Start)
	instance.waitErrFn = sync.OnceValue(cmd.Wait)

	if s.HotReloadable {
		stdin, err := cmd.StdinPipe()
		if err != nil {
			return err
		}
		instance.stdin = stdin
	}

	return nil
}
