package test_env

import (
	"os"
	"testing"
)

func TestEnv(t *testing.T) {
	if os.Getenv("ITEST_ENV_VAR") != "ITEST_ENV_VAR_VALUE" {
		t.Fatal("env var not passed")
	}

	if os.Getenv("ITEST_TEST_TMPDIR") != "$${TEST_TMPDIR}" {
		t.Fatal("TEST_TMPDIR env var replaced")
	}

	if os.Getenv("ITEST_TMPDIR") != "$${TMPDIR}" {
		t.Fatal("TMPDIR env var replaced")
	}

	if os.Getenv("ITEST_SOCKET_DIR") != "$${SOCKET_DIR}" {
		t.Fatal("SOCKET_DIR env var replaced")
	}

}
