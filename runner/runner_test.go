package runner

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"rules_itest/svclib"
)

func TestStartAllSkipsAlreadyRunningServices(t *testing.T) {
	service := &ServiceInstance{
		VersionedServiceSpec: svclib.VersionedServiceSpec{
			ServiceSpec: svclib.ServiceSpec{
				Type:  "service",
				Label: "//:unchanged",
			},
		},
		cmd: &exec.Cmd{
			Args:    []string{"unchanged"},
			Process: &os.Process{Pid: 1},
		},
	}
	runner := &Runner{
		ctx: context.Background(),
		serviceInstances: map[string]*ServiceInstance{
			service.Label: service,
		},
	}

	if _, err := runner.StartAll(make(chan error, 1)); err != nil {
		t.Fatalf("StartAll() returned an error for an already-running service: %v", err)
	}
}
