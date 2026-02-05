#!/bin/bash
set -euo pipefail

# --- begin runfiles.bash initialization v3 ---
# Copy-pasted from the Bazel Bash runfiles library v3.
set -uo pipefail; set +e; f=bazel_tools/tools/bash/runfiles/runfiles.bash
# shellcheck disable=SC1090
source "${RUNFILES_DIR:-/dev/null}/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "${RUNFILES_MANIFEST_FILE:-/dev/null}" | cut -f2- -d' ')" 2>/dev/null || \
  source "$0.runfiles/$f" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  source "$(grep -sm1 "^$f " "$0.exe.runfiles_manifest" | cut -f2- -d' ')" 2>/dev/null || \
  { echo>&2 "ERROR: cannot find $f"; exit 1; }; f=; set -e
# --- end runfiles.bash initialization v3 ---

runfiles_export_envvars

SERVICE_BINARY="$(rlocation _main/shutdown/service_graceful)"
echo "Service binary: $SERVICE_BINARY"

# Set up environment variables that svcinit expects
export SVCINIT_SERVICE_SPECS_RLOCATION_PATH="_main/shutdown/service_graceful.service_specs.json"
export SVCINIT_ALLOW_CONFIGURING_TMPDIR="False"
export SVCINIT_ENABLE_PER_SERVICE_RELOAD="False"
export SVCINIT_KEEP_SERVICES_UP="False"
export SVCINIT_TERSE_OUTPUT="False"

# Unset TEST_TARGET so svcinit doesn't try to run a test binary
unset TEST_TARGET

# Create a temp directory for marker files
MARKER_DIR=$(mktemp -d)
export MARKER_DIR
echo "Marker directory: $MARKER_DIR"

# Run the service binary in background
"$SERVICE_BINARY" 2>&1 &
SERVICE_PID=$!

# Wait for service to start
sleep 1

# Send SIGTERM to trigger graceful shutdown
echo "Sending SIGTERM to service (pid $SERVICE_PID)..."
kill -TERM $SERVICE_PID 2>/dev/null || true

# Wait for service to exit
wait $SERVICE_PID 2>/dev/null || true

# Give a moment for marker files to be written
sleep 1

echo "Checking for marker files in $MARKER_DIR..."

# Check that server started
if [ -f "$MARKER_DIR/server_started" ]; then
    echo "PASS: Service started"
else
    echo "FAIL: Service did not start correctly"
    exit 1
fi

# Check that SIGTERM was received (not SIGKILL)
if [ -f "$MARKER_DIR/signal_SIGTERM" ]; then
    echo "PASS: Service received SIGTERM"
else
    echo "FAIL: Service did not receive SIGTERM - likely received SIGKILL first"
    exit 1
fi

# Check that graceful shutdown completed
if [ -f "$MARKER_DIR/shutdown_complete" ]; then
    echo "PASS: Graceful shutdown completed"
else
    echo "FAIL: Graceful shutdown did not complete"
    exit 1
fi

rm -rf "$MARKER_DIR"
echo "All checks passed!"
