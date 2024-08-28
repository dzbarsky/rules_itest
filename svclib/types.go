package svclib

import "rules_itest/logger"

// Created by Starlark
type ServiceSpec struct {
	// Type can be "service", "task", or "group".
	Type                   string            `json:"type"`
	Label                  string            `json:"label"`
	Args                   []string          `json:"args"`
	Env                    map[string]string `json:"env"`
	Exe                    string            `json:"exe"`
	HttpHealthCheckAddress string            `json:"http_health_check_address"`
	HealthCheck            string            `json:"health_check"`
	HealthCheckLabel       string            `json:"health_check_label"`
	HealthCheckArgs        []string          `json:"health_check_args"`
	HealthCheckInterval    string            `json:"health_check_interval"`
	HealthCheckTimeout     string            `json:"health_check_timeout"`
	VersionFile            string            `json:"version_file"`
	Deps                   []string          `json:"deps"`
	AutoassignPort         bool              `json:"autoassign_port"`
	SoReuseportAware       bool              `json:"so_reuseport_aware"`
	NamedPorts             []string          `json:"named_ports"`
	HotReloadable          bool              `json:"hot_reloadable"`
}

// Our internal representation.
type VersionedServiceSpec struct {
	ServiceSpec
	Version    string
	Color      string
	RusageFile string
}

func (v VersionedServiceSpec) Colorize(label string) string {
	return v.Color + label + logger.Reset
}
