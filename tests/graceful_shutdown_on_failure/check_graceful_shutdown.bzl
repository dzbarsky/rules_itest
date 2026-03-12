"""Rule to verify that services are gracefully shut down when a test fails."""

def _check_graceful_shutdown_impl(ctx):
    # Get the RunEnvironmentInfo from the service_test.
    inner_test = ctx.attr.service_test
    inner_env = inner_test[RunEnvironmentInfo].environment

    inner_test_short_path = ctx.executable.service_test.short_path

    # Generate wrapper script that sets env vars, runs service_test, and checks marker.
    script = ctx.actions.declare_file(ctx.label.name + ".sh")
    env_exports = "\n".join([
        'export %s="%s"' % (k, v)
        for k, v in inner_env.items()
    ])

    ctx.actions.write(
        output = script,
        content = """#!/bin/bash
set -uo pipefail

# Set the env vars that the service_test needs (from RunEnvironmentInfo).
{env_exports}

# Locate the service_test binary via runfiles.
INNER_TEST="$0.runfiles/{workspace}/{inner_test}"

# Run the service_test binary. It is expected to fail because the inner test exits 1.
"$INNER_TEST" || true

# Give a moment for any async cleanup.
sleep 0.5

if [ -f "$TEST_TMPDIR/shutdown_marker" ]; then
    echo "PASS: Service received SIGTERM and shut down gracefully"
    exit 0
else
    echo "FAIL: Service did NOT receive SIGTERM — graceful shutdown was skipped"
    exit 1
fi
""".format(
            env_exports = env_exports,
            workspace = ctx.workspace_name,
            inner_test = inner_test_short_path,
        ),
        is_executable = True,
    )

    runfiles = ctx.runfiles().merge(inner_test[DefaultInfo].default_runfiles)

    return [
        DefaultInfo(
            executable = script,
            runfiles = runfiles,
        ),
    ]

check_graceful_shutdown_test = rule(
    implementation = _check_graceful_shutdown_impl,
    attrs = {
        "service_test": attr.label(
            mandatory = True,
            executable = True,
            cfg = "target",
        ),
    },
    test = True,
)
