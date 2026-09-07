package runner

import (
	"context"
	"errors"
	"strings"
	"testing"

	"rules_itest/svclib"
)

func TestWaitUntilHealthyTaskErrorIncludesLabel(t *testing.T) {
	wantErr := errors.New("task failed")
	service := &ServiceInstance{
		VersionedServiceSpec: svclib.VersionedServiceSpec{ServiceSpec: svclib.ServiceSpec{
			Type:  "task",
			Label: "//example:setup",
		}},
		waitErrFn: func() error { return wantErr },
	}

	err := service.WaitUntilHealthy(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("WaitUntilHealthy() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), service.Label) {
		t.Fatalf("WaitUntilHealthy() error = %q, want service label %q", err, service.Label)
	}
}

func TestWaitUntilHealthyServiceErrorIncludesLabel(t *testing.T) {
	wantErr := errors.New("service failed")
	service := &ServiceInstance{
		VersionedServiceSpec: svclib.VersionedServiceSpec{ServiceSpec: svclib.ServiceSpec{
			Type:                "service",
			Label:               "//example:server",
			HealthCheckInterval: "1ms",
		}},
		runErr: wantErr,
	}

	err := service.WaitUntilHealthy(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("WaitUntilHealthy() error = %v, want wrapped %v", err, wantErr)
	}
	if !strings.Contains(err.Error(), service.Label) {
		t.Fatalf("WaitUntilHealthy() error = %q, want service label %q", err, service.Label)
	}
}

func TestWaitUntilHealthyContextErrorIncludesLabel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := &ServiceInstance{
		VersionedServiceSpec: svclib.VersionedServiceSpec{ServiceSpec: svclib.ServiceSpec{
			Type:                "service",
			Label:               "//example:server",
			HealthCheckInterval: "1ms",
		}},
	}

	err := service.WaitUntilHealthy(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitUntilHealthy() error = %v, want wrapped context cancellation", err)
	}
	if !strings.Contains(err.Error(), service.Label) {
		t.Fatalf("WaitUntilHealthy() error = %q, want service label %q", err, service.Label)
	}
}
