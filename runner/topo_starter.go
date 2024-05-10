package runner

import (
	"context"
	"log"
	"time"

	"rules_itest/runner/topological"
)

type startTask struct {
	serviceInstance  *ServiceInstance
	serviceInstances map[string]*ServiceInstance
}

func (st *startTask) Key() string {
	return st.serviceInstance.Label
}

func (st *startTask) Run(ctx context.Context) error {
	if st.serviceInstance.Type == "group" {
		return nil
	}
	log.Printf("Starting %s %v\n", colorize(st.serviceInstance.VersionedServiceSpec), st.serviceInstance.Args[1:])
	startErr := st.serviceInstance.Start(ctx)
	if startErr != nil {
		return startErr
	}
	return st.serviceInstance.WaitUntilHealthy(ctx)

}

func (st *startTask) Dependents() []topological.Task {
	allTasks := make([]topological.Task, 0, len(st.serviceInstance.Deps))
	for _, label := range st.serviceInstance.Deps {
		allTasks = append(allTasks, &startTask{
			serviceInstance:  st.serviceInstances[label],
			serviceInstances: st.serviceInstances,
		})
	}
	return allTasks
}

func (st *startTask) Duration() time.Duration {
	return st.serviceInstance.StartDuration()
}

func (st *startTask) StartTime() time.Time {
	return st.serviceInstance.StartTime()
}

func newTopologicalStarter(serviceInstances map[string]*ServiceInstance) topological.Runner {
	allTasks := make([]topological.Task, 0, len(serviceInstances))
	for _, serviceInstance := range serviceInstances {
		allTasks = append(allTasks, &startTask{
			serviceInstance:  serviceInstance,
			serviceInstances: serviceInstances,
		})
	}
	return topological.NewRunner(allTasks)
}
