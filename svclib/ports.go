package svclib

import "encoding/json"

type Ports map[string]string

func (p Ports) Set(service string, port string) {
	p[service] = port
}

func (p Ports) Marshal() ([]byte, error) {
	return json.Marshal(p)
}

func (p *Ports) Unmarshal(data []byte) error {
	return json.Unmarshal(data, p)
}
