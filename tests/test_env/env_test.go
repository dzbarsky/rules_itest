package test_env

import (
	"os"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func TestEnv(t *testing.T) {
	if os.Getenv("ITEST_ENV_VAR") != "ITEST_ENV_VAR_VALUE" {
		t.Fatal("env var not passed")
	}
}

func TestRLocation(t *testing.T) {
	filepath, err := runfiles.Rlocation(os.Getenv("ENV_RUNFILE"))
	if err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(filepath)
	if err != nil {
		t.Fatal(err)
	}

	if string(content) != "content" {
		t.Fatal("Content mismatch")
	}
}
