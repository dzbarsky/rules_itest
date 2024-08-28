package main

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func getSpeedyPort(t *testing.T) string {
	cmd := exec.Command(os.Getenv("GET_ASSIGNED_PORT_BIN"), "@@//:_speedy_service")
	port, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}
	return string(port)
}

func getSleepyPort(t *testing.T) string {
	cmd := exec.Command(os.Getenv("GET_ASSIGNED_PORT_BIN"), "@@//:sleepy_service")
	port, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to get port: %v", err)
	}
	return string(port)
}

func TestSvcctlPortRetrieval(t *testing.T) {
	speedyPort := getSpeedyPort(t)

	port := os.Getenv("SVCCTL_PORT")
	if port == "" {
		t.Errorf("SVCCTL_PORT not set")
	}
	svcctlHost := "http://127.0.0.1:" + port

	params := url.Values{}
	params.Add("service", "@@//:_speedy_service")
	resp, err := http.Get(svcctlHost + "/v0/port?" + params.Encode())
	if err != nil {
		t.Errorf("Failed to get port for speedy service: %v", err)
	}
	speedyPort2, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Failed to get port for speedy service: %v", err)
	}
	if string(speedyPort2) != speedyPort {
		t.Errorf("Got port %s, want %s", string(speedyPort2), speedyPort)
	}
}

func TestSvcctl(t *testing.T) {
	speedyPort := getSpeedyPort(t)
	sleepyPort := getSleepyPort(t)

	port := os.Getenv("SVCCTL_PORT")
	if port == "" {
		t.Errorf("SVCCTL_PORT not set")
	}
	svcctlHost := "http://127.0.0.1:" + port

	// Kill speedy service with SIGTERM
	params := url.Values{}
	params.Add("service", "@@//:_speedy_service")
	params.Add("signal", "SIGTERM")
	resp, err := http.Get(svcctlHost + "/v0/kill?" + params.Encode())
	if err != nil {
		t.Errorf("Failed to kill speedy service: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Wait for speedy service to stop with exit code 255
	params = url.Values{}
	params.Add("service", "@@//:_speedy_service")
	resp, err = http.Get(svcctlHost + "/v0/wait?" + params.Encode())
	if err != nil {
		t.Errorf("Failed to wait speedy service: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Terminated by signal, so the exit code should be 255 (except on Windows)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Failed to read response body: %v", err)
	}

	wantCode := "255"
	if runtime.GOOS == "windows" {
		wantCode = "1"
	}

	if string(body) != wantCode {
		t.Errorf("Got exit code %s, want %s", body, wantCode)
	}

	// Health check sleepy service (should be fine)
	params = url.Values{}
	params.Add("service", "@@//:sleepy_service")
	resp, err = http.Get(svcctlHost + "/v0/healthcheck?" + params.Encode())
	if err != nil {
		t.Errorf("Failed to health check sleepy service: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Health check speedy service (should be down)
	params = url.Values{}
	params.Add("service", "@@//:_speedy_service")
	resp, err = http.Get(svcctlHost + "/v0/healthcheck?" + params.Encode())
	if err != nil {
		t.Errorf("Failed to health check speedy service: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}

	// Start speedy service
	params = url.Values{}
	params.Add("service", "@@//:_speedy_service")
	resp, err = http.Get(svcctlHost + "/v0/start?" + params.Encode())
	if err != nil {
		t.Errorf("Failed to start speedy service: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	// Health check speedy service until it is up
	for {
		resp, err = http.Get(svcctlHost + "/v0/healthcheck?" + params.Encode())
		if err != nil {
			t.Errorf("Failed to health check speedy service: %v", err)
		}
		if resp.StatusCode == http.StatusOK {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}

	resp, err = http.Get("http://127.0.0.1:" + speedyPort + "/dob")
	if err != nil {
		t.Errorf("Failed to get dob from speedy service: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	resp, err = http.Get("http://127.0.0.1:" + sleepyPort + "/dob")
	if err != nil {
		t.Errorf("Failed to get dob from sleepy service: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
