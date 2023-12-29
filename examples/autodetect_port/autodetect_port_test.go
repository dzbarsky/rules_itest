package e2e

import (
	"io"
	"net/http"
	"os"
	"testing"
)

func TestServiceHealthcheck(t *testing.T) {
	port := os.Getenv("@@//autodetect_port:service_PORT")

	resp, err := http.DefaultClient.Get("http://127.0.0.1:" + string(port))
	if err != nil {
		t.Fatal(err)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "OK" {
		t.Fatal("did not pass: " + string(body))
	}
}
