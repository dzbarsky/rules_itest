package svclib

type Service struct {
	// Type can be "service" or "task".

	Type                   string            `json:"type"`
	Label                  string            `json:"label"`
	Args                   []string          `json:"args"`
	Env                    map[string]string `json:"env"`
	Exe                    string            `json:"exe"`
	HttpHealthCheckAddress string            `json:"http_health_check_address"`
	VersionFile            string            `json:"version_file"`
	Deps                   []string          `json:"deps"`
}
