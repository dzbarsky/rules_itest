"""Rule to verify that services are gracefully shut down when a test fails."""

load("@rules_shell//shell:sh_test.bzl", "sh_test")

def _gen_check_script_impl(ctx):
    """Generates a shell script that exports RunEnvironmentInfo env vars and checks for graceful shutdown."""
    inner_env = ctx.attr.service_test[RunEnvironmentInfo].environment

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

# Run the service_test binary ($1 is resolved via $(location) by Bazel).
# It is expected to fail because the inner test exits 1.
"$1" || true

# Poll for the shutdown marker with a timeout.
for i in $(seq 1 20); do
    if [ -f "$TEST_TMPDIR/shutdown_marker" ]; then
        echo "PASS: Service received SIGTERM and shut down gracefully"
        exit 0
    fi
    sleep 0.25
done

echo "FAIL: Service did NOT receive SIGTERM — graceful shutdown was skipped"
exit 1
""".format(env_exports = env_exports),
        is_executable = True,
    )

    return [DefaultInfo(files = depset([script]))]

_gen_check_script = rule(
    implementation = _gen_check_script_impl,
    attrs = {
        "service_test": attr.label(
            mandatory = True,
        ),
    },
)

def check_graceful_shutdown_test(name, service_test, **kwargs):
    """Verifies that services receive SIGTERM even when the inner test fails."""
    _gen_check_script(
        name = name + "_script",
        service_test = service_test,
        testonly = True,
        tags = ["manual"],
    )

    sh_test(
        name = name,
        srcs = [":" + name + "_script"],
        args = ["$(location %s)" % service_test],
        data = [service_test],
        **kwargs
    )
