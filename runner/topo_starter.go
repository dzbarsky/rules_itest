package runner

import (
	"fmt"
	"rules_itest/runner/topological"
	"time"
)

type startTask struct {
	serviceInstance  *ServiceInstance
	serviceInstances map[string]*ServiceInstance
}

func (st *startTask) Key() string {
	return st.serviceInstance.Label
}

func (st *startTask) Run() error {
	fmt.Println("starting " + st.serviceInstance.Label)
	startErr := st.serviceInstance.Start()
	if startErr != nil {
		return startErr
	}
	fmt.Println("waiting for " + st.serviceInstance.Label)
	return st.serviceInstance.WaitUntilHealthy()
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
