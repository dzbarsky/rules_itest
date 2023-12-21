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

_svcinit_attr = {
    "_svcinit": attr.label(default = "//cmd/svcinit", executable = True, cfg = "target"),
}

_itest_binary_attrs = {
    "exe": attr.label(mandatory = True, executable = True, cfg = "target"),
    "env": attr.string_dict(),
    "data": attr.label_list(allow_files = True),
    "deps": attr.label_list(providers = [ServiceGroupInfo]),
} | _svcinit_attr

def _itest_binary_impl(ctx, extra_service_spec_kwargs, extra_exe_runfiles = []):
    version_file = ctx.actions.declare_file(ctx.label.name + ".version")

    exe_runfiles = [ctx.attr.exe.default_runfiles] + extra_exe_runfiles

    version_file_deps = ctx.files.data + ctx.files.exe
    version_file_deps_trans = [runfiles.files for runfiles in exe_runfiles]

    _create_version_file(
        ctx,
        depset(direct = version_file_deps, transitive = version_file_deps_trans),
        output = version_file,
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
    ] + exe_runfiles)

    return [
        DefaultInfo(runfiles = runfiles),
        ServiceGroupInfo(services = services),
    ]

def _itest_service_impl(ctx):
    extra_service_spec_kwargs = {
        "type": "service",
        "http_health_check_address": ctx.attr.http_health_check_address,
        "autodetect_port": ctx.attr.autodetect_port,
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
    "autodetect_port": attr.bool(),
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
    ] + [ctx.attr._svcinit.default_runfiles])

    return [
        DefaultInfo(runfiles = runfiles),
        ServiceGroupInfo(services = services),
    ]

_itest_service_group_attrs = {
    "services": attr.label_list(providers = [ServiceGroupInfo]),
    "data": attr.label_list(allow_files = True),
    "env": attr.string_dict(),
} | _svcinit_attr

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
    ] + [ctx.attr._svcinit.default_runfiles, ctx.attr.test.default_runfiles])

    return [
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

def _create_version_file(ctx, inputs, output):
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
