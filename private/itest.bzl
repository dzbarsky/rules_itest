""" Rules for running services in integration tests. """

ServiceGroupInfo = provider(
    doc = "Info about a service group",
    fields = {
        "services": "Dict of services/tasks",
    },
)

def _collect_services(deps):
    services = {}
    for dep in deps:
        services |= dep[ServiceGroupInfo].services
    return services

def _run_environment(ctx):
    return RunEnvironmentInfo(environment = {
        "GET_ASSIGNED_PORT_BIN": ctx.file._get_assigned_port.short_path,
    })

_svcinit_attrs = {
    "_svcinit": attr.label(
        default = "//cmd/svcinit",
        executable = True,
        cfg = "target",
    ),
    "_get_assigned_port": attr.label(
        default = "//cmd/get_assigned_port",
        allow_single_file = True,
        executable = True,
        cfg = "target",
    ),
    "_prepare_version_file": attr.label(
        default = "//cmd/prepare_version_file",
        allow_single_file = True,
        executable = True,
        cfg = "exec",
    ),
}

_itest_binary_attrs = {
    "exe": attr.label(mandatory = True, executable = True, cfg = "target"),
    "env": attr.string_dict(),
    "data": attr.label_list(allow_files = True),
    "deps": attr.label_list(providers = [ServiceGroupInfo]),
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

    service = struct(
        label = str(ctx.label),
        exe = ctx.executable.exe.short_path,
        args = args,
        env = env,
        deps = [str(dep.label) for dep in ctx.attr.deps],
        version_file = version_file.short_path,
        **extra_service_spec_kwargs
    )

    services = _collect_services(ctx.attr.deps)
    services[service.label] = service

    service_specs_file = _create_svcinit_actions(ctx, services)

    runfiles = ctx.runfiles(ctx.files.data + [service_specs_file, version_file])
    runfiles = runfiles.merge_all([
        service.default_runfiles
        for service in ctx.attr.deps
    ] + [
        ctx.attr._svcinit.default_runfiles,
        ctx.attr._get_assigned_port.default_runfiles,
    ] + exe_runfiles)

    return [
        _run_environment(ctx),
        DefaultInfo(runfiles = runfiles),
        ServiceGroupInfo(services = services),
    ]

def _itest_service_impl(ctx):
    extra_service_spec_kwargs = {
        "type": "service",
        "http_health_check_address": ctx.attr.http_health_check_address,
        "autoassign_port": ctx.attr.autoassign_port,
    }
    extra_exe_runfiles = []

    if ctx.attr.health_check:
        extra_service_spec_kwargs["health_check"] = ctx.executable.health_check.short_path
        extra_exe_runfiles.extend([
            ctx.attr.health_check.default_runfiles,
            ctx.attr.health_check.data_runfiles,
        ])

    return _itest_binary_impl(ctx, extra_service_spec_kwargs, extra_exe_runfiles)

_itest_service_attrs = _itest_binary_attrs | {
    "http_health_check_address": attr.string(),
    # Note, autoassigning a port is a little racy. If you can stick to hardcoded ports and network namespace, you should prefer that.
    "autoassign_port": attr.bool(),
    "health_check": attr.label(cfg = "target", mandatory = False, executable = True),
}

itest_service = rule(
    implementation = _itest_service_impl,
    attrs = _itest_service_attrs,
    executable = True,
)

def _itest_task_impl(ctx):
    return _itest_binary_impl(ctx, {
        "type": "task",
    })

itest_task = rule(
    implementation = _itest_task_impl,
    attrs = _itest_binary_attrs,
    executable = True,
)

def _itest_service_group_impl(ctx):
    services = _collect_services(ctx.attr.services)
    service_specs_file = _create_svcinit_actions(ctx, services)

    runfiles = ctx.runfiles(ctx.files.data + [service_specs_file]).merge_all([
        service.default_runfiles
        for service in ctx.attr.services
    ] + [
        ctx.attr._svcinit.default_runfiles,
        ctx.attr._get_assigned_port.default_runfiles,
    ])

    return [
        _run_environment(ctx),
        DefaultInfo(runfiles = runfiles),
        ServiceGroupInfo(services = services),
    ]

_itest_service_group_attrs = {
    "services": attr.label_list(providers = [ServiceGroupInfo]),
    "data": attr.label_list(allow_files = True),
    "env": attr.string_dict(),
} | _svcinit_attrs

itest_service_group = rule(
    implementation = _itest_service_group_impl,
    attrs = _itest_service_group_attrs,
    executable = True,
)

def _create_svcinit_actions(ctx, services, extra_svcinit_args = ""):
    # Avoid expanding during analysis phase.
    service_content = ctx.actions.args()
    service_content.set_param_file_format("multiline")
    service_content.add_all([services], map_each = json.encode)

    service_specs_file = ctx.actions.declare_file(ctx.label.name + ".service_specs.json")
    ctx.actions.write(
        output = service_specs_file,
        content = service_content,
    )

    ctx.actions.write(
        output = ctx.outputs.executable,
        content = 'exec {svcinit_path} -svc.specs-path={service_specs_path} {extra_svcinit_args} "$@"'.format(
            svcinit_path = ctx.executable._svcinit.short_path,
            service_specs_path = service_specs_file.short_path,
            extra_svcinit_args = extra_svcinit_args,
        ),
    )

    return service_specs_file

def _service_test_impl(ctx):
    extra_svcinit_args = [ctx.executable.test.short_path]
    service_specs_file = _create_svcinit_actions(
        ctx,
        _collect_services(ctx.attr.services),
        extra_svcinit_args = " ".join(extra_svcinit_args),
    )

    runfiles = ctx.runfiles(ctx.files.data + [service_specs_file])
    runfiles = runfiles.merge_all([
        service.default_runfiles
        for service in ctx.attr.services
    ] + [
        ctx.attr._svcinit.default_runfiles,
        ctx.attr._get_assigned_port.default_runfiles,
        ctx.attr.test.default_runfiles,
    ])

    return [
        _run_environment(ctx),
        DefaultInfo(runfiles = runfiles),
        coverage_common.instrumented_files_info(ctx, dependency_attributes = ["test"]),
    ]

_service_test_attrs = {
    "test": attr.label(cfg = "target", mandatory = False, executable = True),
} | _itest_service_group_attrs

service_test = rule(
    implementation = _service_test_impl,
    attrs = _service_test_attrs,
    test = True,
)

def _create_version_file(ctx, inputs):
    # This setup is extremely hacky.
    # We want a notion of "service version" for per-service hot-reloading that we can use with ibazel.
    # However, computing an actual hash is needlessly slow; so we use `date` instead.
    # This version is only used for iterating/hot-reloading, it does not affect actual test execution.
    # So we want to make sure we don't affect the cache key.
    # The way we accomplish this is by feeding the time-based version file into a secondary action.
    # The secondary action marks it as an unused_input so it gets excluded from the cache key.
    # It also emits a symlink to the real on-disk version file (sandbox escape) so the second action is cacheable.
    # This probably won't work with RBE, but is fixable by generating a constant version file in that case.

    raw_version_file = ctx.actions.declare_file(ctx.label.name + ".raw_version")

    ctx.actions.run_shell(
        inputs = inputs,
        tools = [],  # Ensure inputs in the host configuration are not treated specially.
        outputs = [raw_version_file],
        command = "/bin/date > {}".format(
            raw_version_file.path,
        ),
        mnemonic = "SvcVersionFile",
        # disable remote cache and sandbox, since the output is not stable given the inputs
        # additionally, running this action in the sandbox is way too expensive
        execution_requirements = {"local": "1"},
    )

    version_file = ctx.actions.declare_symlink(ctx.label.name + ".version")
    unused_inputs_file = ctx.actions.declare_file(ctx.label.name + ".version_file_unused_inputs")

    ctx.actions.run(
        inputs = [raw_version_file],
        unused_inputs_list = unused_inputs_file,
        outputs = [version_file, unused_inputs_file],
        executable = ctx.executable._prepare_version_file,
        arguments = [raw_version_file.path, version_file.path, unused_inputs_file.path],
    )

    return version_file
