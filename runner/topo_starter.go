package runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
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

func (st *startTask) Run() error {
	fmt.Println("starting " + st.serviceInstance.Label)
	startErr := st.serviceInstance.Start()
	if startErr != nil {
		return startErr
	}
	fmt.Printf("waiting for %s (pid %d)\n", st.serviceInstance.Label, st.serviceInstance.Process.Pid)

	output, err := exec.Command("lsof",
		// Network connections
		"-i",
		// AND
		"-a",
		// Owned by our pid
		"-p", strconv.Itoa(st.serviceInstance.Process.Pid),
		"-F", "n").CombinedOutput()
	if err != nil {
		return err
	}
	// Output looks like so:
	//
	// p24051
	// f3
	// nlocalhost:52263
	parts := strings.Split(string(output), ":")
	port := parts[1]
	// Trim trailing \n
	port = port[:len(port)-1]

	// It's OK to mutate our spec since we have made a copy of it.
	st.serviceInstance.HttpHealthCheckAddress = strings.ReplaceAll(
		st.serviceInstance.HttpHealthCheckAddress, "$PORT", port)

	err = st.serviceInstance.WaitUntilHealthy()
	if err != nil {
		return err
	}

	// Note, this can cause collisions. So be careful!
	portFile := strings.ReplaceAll(st.serviceInstance.Label, "/", "_")

	return os.WriteFile(
		filepath.Join(os.Getenv("TEST_TMPDIR"), portFile), []byte(port), 0600)
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
