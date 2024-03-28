""" Rules for running services in integration tests. """

load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")

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
        allow_single_file = True,
        executable = True,
        cfg = "target",
    ),
    "_enable_per_service_reload": attr.label(
        default = "//:enable_per_service_reload",
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

    if version_file:
        extra_service_spec_kwargs["version_file"] = version_file.short_path

    service = struct(
        label = str(ctx.label),
        exe = ctx.executable.exe.short_path,
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
        _run_environment(ctx),
        DefaultInfo(runfiles = runfiles),
        ServiceGroupInfo(services = services),
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
        extra_service_spec_kwargs["health_check"] = ctx.executable.health_check.short_path
        extra_exe_runfiles.append(ctx.attr.health_check.default_runfiles)

    return _itest_binary_impl(ctx, extra_service_spec_kwargs, extra_exe_runfiles)

_itest_service_attrs = _itest_binary_attrs | {
    "http_health_check_address": attr.string(),
    # Note, autoassigning a port is a little racy. If you can stick to hardcoded ports and network namespace, you should prefer that.
    "autoassign_port": attr.bool(),
    "health_check": attr.label(cfg = "target", mandatory = False, executable = True),
    "hot_reloadable": attr.bool(),
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

    runfiles = ctx.runfiles(ctx.files.data + [service_specs_file])
    runfiles = runfiles.merge_all(_services_runfiles(ctx))

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
        content = 'exec {svcinit_path} -svc.specs-path={service_specs_path} -svc.enable-hot-reload={enable_per_service_reload} {extra_svcinit_args} "$@"'.format(
            svcinit_path = ctx.executable._svcinit.short_path,
            service_specs_path = service_specs_file.short_path,
            enable_per_service_reload = ctx.attr._enable_per_service_reload[BuildSettingInfo].value,
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
    runfiles = runfiles.merge_all(_services_runfiles(ctx) + [
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
