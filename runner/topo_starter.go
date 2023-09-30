package runner

import (
	"fmt"
	"rules_itest/runner/topological"
	"time"
)

type startTask struct {
	svcCommand  *ServiceCommand
	svcCommands map[string]*ServiceCommand
}

func (st *startTask) Key() string {
	return st.svcCommand.Label
}

func (st *startTask) Run() error {
	fmt.Println("starting " + st.svcCommand.Label)
	startErr := st.svcCommand.Start()
	if startErr != nil {
		return startErr
	}
	fmt.Println("waiting for " + st.svcCommand.Label)
	return st.svcCommand.WaitUntilHealthy()
}

func (st *startTask) Dependents() []topological.Task {
	allTasks := make([]topological.Task, 0, len(st.svcCommand.Deps))
	for _, label := range st.svcCommand.Deps {
		allTasks = append(allTasks, &startTask{
			svcCommand:  st.svcCommands[label],
			svcCommands: st.svcCommands,
		})
	}
	return allTasks
}

func (st *startTask) Duration() time.Duration {
	return st.svcCommand.StartDuration()
}

func (st *startTask) StartTime() time.Time {
	return st.svcCommand.StartTime()
}

func newTopologicalStarter(svcCommands map[string]*ServiceCommand) topological.Runner {
	allTasks := make([]topological.Task, 0, len(svcCommands))
	for _, svcCommand := range svcCommands {
		allTasks = append(allTasks, &startTask{
			svcCommand:  svcCommand,
			svcCommands: svcCommands,
		})
	}
	return topological.NewRunner(allTasks)
}
