package test_env

import (
	"os"
	"testing"
)

func TestEnv(t *testing.T) {
	if os.Getenv("ITEST_ENV_VAR") != "ITEST_ENV_VAR_VALUE" {
		t.Fatal("env var not passed")
	}
}
