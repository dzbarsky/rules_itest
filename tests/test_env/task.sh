#!/usr/bin/env bash
set -eux

if [ "$ITEST_ENV_VAR" != "ITEST_ENV_VAR_VALUE" ]; then
  echo "Missing expected env var"
  exit 1
fi;
