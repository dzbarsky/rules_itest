""" Rules for running services in integration tests. """

ServiceGroupInfo = provider(
    doc = "Info about a service group",
    fields = {
        "services": "Dict of services/tasks",
    },
)

def _collect_services(ctx):
    services = {}
    for dep in ctx.attr.services:
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

def _itest_binary_impl(ctx, extra_service_definition_kwargs):
    version_file = ctx.actions.declare_file(ctx.label.name + ".version")

    version_file_deps = ctx.files.data + ctx.files.exe
    version_file_deps_trans = [ctx.attr.exe.default_runfiles.files]

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
        deps = ctx.attr.deps,
        version_file = version_file.short_path,
        **extra_service_definition_kwargs
    )

    services = {service.label: service}
    for dep in ctx.attr.deps:
        services |= dep[ServiceGroupInfo].services

    services, service_defs_file = _create_svcinit_actions(ctx, services)

    runfiles = ctx.runfiles(ctx.attr.data + [service_defs_file, version_file])
    runfiles = runfiles.merge_all([
        ctx.attr.exe.default_runfiles,
        ctx.attr._svcinit.default_runfiles,
    ])

    return [
        DefaultInfo(runfiles = runfiles),
        ServiceGroupInfo(services = services),
    ]

def _itest_service_impl(ctx):
    return _itest_binary_impl(ctx, {
        "type": "service",
        "http_health_check_address": ctx.attr.http_health_check_address,
    })

_itest_service_attrs = _itest_binary_attrs | {
    "http_health_check_address": attr.string(),
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
    services, service_defs_file = _create_svcinit_actions(
        ctx,
        _collect_services(ctx),
    )

    runfiles = ctx.runfiles(ctx.attr.data + [service_defs_file]).merge_all([
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

    service_defs_file = ctx.actions.declare_file(ctx.label.name + ".service_defs.json")
    ctx.actions.write(
        output = service_defs_file,
        content = service_content,
    )

    ctx.actions.write(
        output = ctx.outputs.executable,
        content = 'exec {svcinit_path} -svc.definitions-path={service_definitions} {extra_svcinit_args} "$@"'.format(
            svcinit_path = ctx.executable._svcinit.short_path,
            service_definitions = service_defs_file.short_path,
            extra_svcinit_args = extra_svcinit_args,
        ),
    )

    return services, service_defs_file

def _service_test_impl(ctx):
    extra_svcinit_args = ["--svc.test-label", str(ctx.label), ctx.executable.test.short_path]
    _, service_defs_file = _create_svcinit_actions(
        ctx,
        _collect_services(ctx),
        extra_svcinit_args = " ".join(extra_svcinit_args),
    )

    runfiles = ctx.runfiles(ctx.attr.data + [service_defs_file])
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
