package svcctl

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
)

type SvcctlClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewSvcctlClient(baseURL string, client *http.Client) *SvcctlClient {
	return &SvcctlClient{
		baseURL:    baseURL,
		httpClient: client,
	}
}

func (c *SvcctlClient) StartService(ctx context.Context, service string, waitForHealthy bool) error {
	q := url.Values{}
	q.Set("service", service)
	if waitForHealthy {
		q.Set("wait_for_healthy", "1")
	} else {
		q.Set("wait_for_healthy", "0")
	}

	log.Printf(c.baseURL+"/v0/start?" + q.Encode())

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v0/start?" + q.Encode(), nil)
	if err != nil {
		return err
	}

	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to start speedy service: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	return nil
}

func (c *SvcctlClient) WaitForService(ctx context.Context, service string) error {
	q := url.Values{}
	q.Set("service", service)

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v0/wait?" + q.Encode(), nil)
	if err != nil {
		return err
	}

	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Failed to start speedy service: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Got status code %d, want %d", resp.StatusCode, http.StatusOK)
	}

	return nil
}

func (c *SvcctlClient) HealthCheck(ctx context.Context, service string) (int, error) {
	q := url.Values{}
	q.Set("service", service)

	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/v0/healthcheck?" + q.Encode(), nil)
	if err != nil {
		return -1, err
	}

	req.WithContext(ctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return -1, err
	}

	return resp.StatusCode, nil
}