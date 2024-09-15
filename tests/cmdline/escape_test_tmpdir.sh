#!/usr/bin/env bash
set -eux

got="$1"

echo "$got"

# shellcheck disable=SC2016
if [[ "$got" == \$* ]]; then
  echo "Received: '$got'"
  exit 1
fi;

