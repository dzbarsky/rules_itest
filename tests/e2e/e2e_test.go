package e2e

import (
	"io"
	"net/http"
	"testing"
)

func TestServiceHealthcheck(t *testing.T) {
	resp, err := http.DefaultClient.Get("http://127.0.0.1:8001")
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
