package runner

import (
	"context"
	"time"

	"rules_itest/runner/topological"
)

type RunFunc func(ctx context.Context, service *ServiceInstance) error

type topoTask struct {
	serviceInstance  *ServiceInstance
	serviceInstances map[string]*ServiceInstance
	runFunc          RunFunc
}

func (st *topoTask) Key() string {
	return st.serviceInstance.Label
}

func (st *topoTask) Run(ctx context.Context) error {
	return st.runFunc(ctx, st.serviceInstance)
}

func (st *topoTask) Dependents() []topological.Task {
	allTasks := make([]topological.Task, 0, len(st.serviceInstance.Deps))
	for _, label := range st.serviceInstance.Deps {
		allTasks = append(allTasks, &topoTask{
			serviceInstance:  st.serviceInstances[label],
			serviceInstances: st.serviceInstances,
			runFunc:          st.runFunc,
		})
	}
	return allTasks
}

func (st *topoTask) Duration() time.Duration {
	return st.serviceInstance.StartDuration()
}

func (st *topoTask) StartTime() time.Time {
	return st.serviceInstance.StartTime()
}

func allTasks(serviceInstances map[string]*ServiceInstance, runFunc RunFunc) []topological.Task {
	allTasks := make([]topological.Task, 0, len(serviceInstances))
	for _, serviceInstance := range serviceInstances {
		allTasks = append(allTasks, &topoTask{
			serviceInstance:  serviceInstance,
			serviceInstances: serviceInstances,
			runFunc:          runFunc,
		})
	}
	return allTasks
}
