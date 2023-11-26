package svclib

// Created by Starlark
type ServiceSpec struct {
	// Type can be "service" or "task".

	Type                   string            `json:"type"`
	Label                  string            `json:"label"`
	Args                   []string          `json:"args"`
	Env                    map[string]string `json:"env"`
	Exe                    string            `json:"exe"`
	HttpHealthCheckAddress string            `json:"http_health_check_address"`
	HealthCheck            string            `json:"health_check"`
	VersionFile            string            `json:"version_file"`
	Deps                   []string          `json:"deps"`
	AutodetectPort         bool              `json:"autodetect_port"`
}

// Our internal representation.
type VersionedServiceSpec struct {
	ServiceSpec
	Version string
}
