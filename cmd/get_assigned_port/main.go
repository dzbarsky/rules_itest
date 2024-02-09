package main

import (
	"encoding/json"
	"os"
)

func main() {
	ports := map[string]string{}
	err := json.Unmarshal([]byte(os.Getenv("ASSIGNED_PORTS")), &ports)
	if err != nil {
		panic(err)
	}

	os.Stdout.Write([]byte(ports[os.Args[1]]))
}
