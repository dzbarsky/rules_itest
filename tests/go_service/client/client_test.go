package client

import (
	"net/http"
	"os"
	"testing"
)

func TestService(t *testing.T) {
	port := os.Getenv("TEST_PORT")
	resp, err := http.Get("http://127.0.0.1:" + port)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatal("bad status")
	}
}
