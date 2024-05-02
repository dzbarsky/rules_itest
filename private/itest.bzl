"""
# Rules for running services in integration tests.

This ruleset supports [ibazel](https://github.com/bazelbuild/bazel-watcher) when using `bazel run`.
As a UX optimization, the service manager is able to restart only the modified services, instead of all services,
when it receives the reload notification from ibazel. This capability depends on a cache-busting input, so it is hidden
behind an an extra CLI flag, like so:
`ibazel run @rules_itest//:enable_per_service_reload //path/to:target`

In addition, you can set the `hot_reloadable` attribute on an `itest_service`, in that case the service manager will
forward the ibazel hot-reload notification over stdin instead of restarting the service.
"""

load("@aspect_bazel_lib//lib:paths.bzl", "to_rlocation_path")
load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")

_ServiceGroupInfo = provider(
    doc = "Info about a service group",
    fields = {
        "services": "Dict of services/tasks",
    },
)

def _collect_services(deps):
    services = {}
    for dep in deps:
        services |= dep[_ServiceGroupInfo].services
    return services

def _run_environment(ctx, service_specs_file):
    return {
        "SVCINIT_SERVICE_SPECS_RLOCATION_PATH": to_rlocation_path(ctx, service_specs_file),
        "SVCINIT_ENABLE_PER_SERVICE_RELOAD": str(ctx.attr._enable_per_service_reload[BuildSettingInfo].value),
        "SVCINIT_GET_ASSIGNED_PORT_BIN_RLOCATION_PATH": to_rlocation_path(ctx, ctx.executable._get_assigned_port),
    }

def _services_runfiles(ctx, services_attr_name = "services"):
    return [
        service.default_runfiles
        for service in getattr(ctx.attr, services_attr_name)
    ] + [
        ctx.attr._svcinit.default_runfiles,
        ctx.attr._get_assigned_port.default_runfiles,
    ]

_svcinit_attrs = {
    "_svcinit": attr.label(
        default = "//cmd/svcinit",
        executable = True,
        cfg = "target",
    ),
    "_get_assigned_port": attr.label(
        default = "//cmd/get_assigned_port",
        executable = True,
        cfg = "target",
    ),
    "_enable_per_service_reload": attr.label(
        default = "//:enable_per_service_reload",
    ),
}

_itest_binary_attrs = {
    "exe": attr.label(
        mandatory = True,
        executable = True,
        cfg = "target",
        doc = "The binary target to run.",
    ),
    "env": attr.string_dict(
        doc = "The service manager will merge these variables into the environment when spawning the underlying binary.",
    ),
    "data": attr.label_list(allow_files = True),
    "deps": attr.label_list(
        providers = [_ServiceGroupInfo],
        doc = "Services/tasks that must be started before this service/task can be started. Can be `itest_service`, `itest_task`, or `itest_service_group`.",
    ),
} | _svcinit_attrs

def _itest_binary_impl(ctx, extra_service_spec_kwargs, extra_exe_runfiles = []):
    exe_runfiles = [ctx.attr.exe.default_runfiles] + extra_exe_runfiles

    version_file_deps = ctx.files.data + ctx.files.exe
    version_file_deps_trans = [runfiles.files for runfiles in exe_runfiles]

    version_file = _create_version_file(
        ctx,
        depset(direct = version_file_deps, transitive = version_file_deps_trans),
    )

    args = [
        ctx.expand_location(arg, targets = ctx.attr.data)
        for arg in ctx.attr.args
    ]

    env = {
        var: ctx.expand_location(val, targets = ctx.attr.data)
        for (var, val) in ctx.attr.env.items()
    }

    if version_file:
        extra_service_spec_kwargs["version_file"] = to_rlocation_path(ctx, version_file)

    service = struct(
        label = str(ctx.label),
        exe = to_rlocation_path(ctx, ctx.executable.exe),
        args = args,
        env = env,
        deps = [str(dep.label) for dep in ctx.attr.deps],
        **extra_service_spec_kwargs
    )

    services = _collect_services(ctx.attr.deps)
    services[service.label] = service

    service_specs_file = _create_svcinit_actions(ctx, services)

    direct_runfiles = ctx.files.data + [service_specs_file]
    if version_file:
        direct_runfiles.append(version_file)

    runfiles = ctx.runfiles(direct_runfiles)
    runfiles = runfiles.merge_all(_services_runfiles(ctx, "deps") + exe_runfiles)

    return [
        RunEnvironmentInfo(environment = _run_environment(ctx, service_specs_file)),
        DefaultInfo(runfiles = runfiles),
        _ServiceGroupInfo(services = services),
    ]

def _itest_service_impl(ctx):
    extra_service_spec_kwargs = {
        "type": "service",
        "http_health_check_address": ctx.attr.http_health_check_address,
        "autoassign_port": ctx.attr.autoassign_port,
        "hot_reloadable": ctx.attr.hot_reloadable,
    }
    extra_exe_runfiles = []

    if ctx.attr.health_check:
        extra_service_spec_kwargs["health_check"] = to_rlocation_path(ctx, ctx.executable.health_check)
        extra_exe_runfiles.append(ctx.attr.health_check.default_runfiles)

    return _itest_binary_impl(ctx, extra_service_spec_kwargs, extra_exe_runfiles)

_itest_service_attrs = _itest_binary_attrs | {
    # Note, autoassigning a port is a little racy. If you can stick to hardcoded ports and network namespace, you should prefer that.
    "autoassign_port": attr.bool(
        doc = """If true, the service manager will pick a free port and assign it to the service.
        The port can be accessed by using `$${PORT}` in the service's args.
        
        The port is also acessible in the service-port mapping, which is a JSON string -> int map in the env var `ASSIGNED_PORTS`.
        For example, a port can be retrieved with the following JS code:
        `JSON.parse(process.env["ASSIGNED_PORTS"])["@@//label/for:service"]`.
        
        Alternately, the env will also contain the location of a binary that can return the port, for contexts without a readily-accessible JSON parser.
        For example, the following Bash command: 
        `PORT=$($GET_ASSIGNED_PORT_BIN @@//label/for:service)`""",
    ),
    "health_check": attr.label(
        cfg = "target",
        mandatory = False,
        executable = True,
        doc = """If set, the service manager will execute this binary to check if the service came up in a healthy state.
        This check will be retried until it exits with a 0 exit code. When used in conjunction with autoassigned ports, use
        one of the methods described in `autoassign_port` to locate the service.""",
    ),
    "hot_reloadable": attr.bool(
        doc = """If set to True, the service manager will propagate ibazel's reload notificaiton over stdin instead of restarting the service.
        See the ruleset docstring for more info on using ibazel""",
    ),
    "http_health_check_address": attr.string(
        doc = """If set, the service manager will send an HTTP request to this address to check if the service came up in a healthy state.
        This check will be retried until it returns a 200 HTTP code. When used in conjunction with autoassigned ports, `$${PORT}` can be used in the address.
        Example: `http_health_check_address = "http://localhost:$${PORT}",`""",
    ),
}

itest_service = rule(
    implementation = _itest_service_impl,
    attrs = _itest_service_attrs,
    executable = True,
    doc = "An itest_service is a binary that is intended to run for the duration of the integration test. Examples include databases, HTTP/RPC servers, queue consumers, external service mocks, etc.",
)

def _itest_task_impl(ctx):
    return _itest_binary_impl(ctx, {
        "type": "task",
    })

itest_task = rule(
    implementation = _itest_task_impl,
    attrs = _itest_binary_attrs,
    executable = True,
    doc = """A task is a one-shot (not long-running binary) that is intended to be executed as part of the itest scenario creation.
Examples include: filesystem setup, dynamic config file generation (especially if it depends on ports), DB migrations or seed data creation""",
)

def _itest_service_group_impl(ctx):
    services = _collect_services(ctx.attr.services)
    service_specs_file = _create_svcinit_actions(ctx, services)

    runfiles = ctx.runfiles([service_specs_file])
    runfiles = runfiles.merge_all(_services_runfiles(ctx))

    return [
        RunEnvironmentInfo(environment = _run_environment(ctx, service_specs_file)),
        DefaultInfo(runfiles = runfiles),
        _ServiceGroupInfo(services = services),
    ]

_itest_service_group_attrs = {
    "services": attr.label_list(
        providers = [_ServiceGroupInfo],
        doc = "Services/tasks that comprise this group. Can be `itest_service`, `itest_task`, or `itest_service_group`.",
    ),
} | _svcinit_attrs

itest_service_group = rule(
    implementation = _itest_service_group_impl,
    attrs = _itest_service_group_attrs,
    executable = True,
    doc = """A service group is a collection of services/tasks.

It serves as a convenient way for a downstream target to depend on multiple services with a single label, without
forcing the services within the group to define a specific startup ordering with their `deps`.

It is also useful to bring up multiple services with a single `bazel run` command, which is useful for creating
dev environments.""",
)

def _create_svcinit_actions(ctx, services):
    ctx.actions.symlink(
        output = ctx.outputs.executable,
        target_file = ctx.executable._svcinit,
    )

    # Avoid expanding during analysis phase.
    service_content = ctx.actions.args()
    service_content.set_param_file_format("multiline")
    service_content.add_all([services], map_each = json.encode)

    service_specs_file = ctx.actions.declare_file(ctx.label.name + ".service_specs.json")
    ctx.actions.write(
        output = service_specs_file,
        content = service_content,
    )

    return service_specs_file

def _service_test_impl(ctx):
    service_specs_file = _create_svcinit_actions(
        ctx,
        _collect_services(ctx.attr.services),
    )

    runfiles = ctx.runfiles([service_specs_file])
    runfiles = runfiles.merge_all(_services_runfiles(ctx) + [
        ctx.attr.test.default_runfiles,
    ])

    env = _run_environment(ctx, service_specs_file)
    env["SVCINIT_TEST_RLOCATION_PATH"] = to_rlocation_path(ctx, ctx.executable.test)

    return [
        RunEnvironmentInfo(environment = env),
        DefaultInfo(runfiles = runfiles),
        coverage_common.instrumented_files_info(ctx, dependency_attributes = ["test"]),
    ]

_service_test_attrs = {
    "test": attr.label(
        cfg = "target",
        executable = True,
        doc = "The underlying test target to execute once the services have been brought up and healthchecked.",
    ),
    "env": attr.string_dict(
        doc = "The service manager will merge these variables into the environment when spawning the underlying binary.",
    ),
    "data": attr.label_list(allow_files = True),
} | _itest_service_group_attrs

service_test = rule(
    implementation = _service_test_impl,
    attrs = _service_test_attrs,
    test = True,
    doc = """Brings up a set of services/tasks and runs a test target against them.
    
This can be used to customize which services a particular test needs while being able to bring them up in an easy and consistent way.

Example usage:
```
go_test(
    name = "_example_test_no_services",
    srcs = [..],
    tags = ["manual"],
)

service_test(
    name = "example_test",
    test = ":_example_test_no_services",
    services = [
        "//services/mysql",
        ...
    ],
)
```

Typically this would be wrapped into a macro.""",
)

def _create_version_file(ctx, inputs):
    if not ctx.attr._enable_per_service_reload[BuildSettingInfo].value:
        return None

    output = ctx.actions.declare_file(ctx.label.name + ".version")

    ctx.actions.run_shell(
        inputs = inputs,
        tools = [],  # Ensure inputs in the host configuration are not treated specially.
        outputs = [output],
        command = "/bin/date > {}".format(
            output.path,
        ),
        mnemonic = "SvcVersionFile",
        # disable remote cache and sandbox, since the output is not stable given the inputs
        # additionally, running this action in the sandbox is way too expensive
        execution_requirements = {"local": "1"},
    )

    return output
