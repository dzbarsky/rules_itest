""" Rules for running services in integration tests. """

load("@bazel_skylib//rules:common_settings.bzl", "int_flag")
load(
    "//private:itest.bzl",
    _itest_service = "itest_service",
    _itest_service_group = "itest_service_group",
    _itest_task = "itest_task",
    _service_test = "service_test",
)

def port(label):
    return "$${%s}" % (str(native.package_relative_label(label)))

def named_port(label, name):
    return port(label + "." + name)

def itest_service(name, tags = [], hygienic = True, named_ports = [], **kwargs):
    if "port" in kwargs:
        fail("Do not specify `port`, instead set it via the `%s` flag" % (name + ".port"))

    int_flag(
        name = name + ".port",
        build_setting_default = 0,
    )

    named_ports_attr = {}
    for named_port in named_ports:
        named_port_label = name + "." + named_port
        named_ports_attr[named_port_label] = named_port

        int_flag(
            name = named_port_label,
            build_setting_default = 0,
        )

    _itest_service(
        name = name,
        tags = tags + ["ibazel_notify_changes"],
        port = name + ".port",
        named_ports = named_ports_attr,
        **kwargs
    )

    if hygienic:
        _hygiene_test(
            name = name,
            tags = tags,
        )

def itest_service_group(name, tags = [], hygienic = True, **kwargs):
    _itest_service_group(
        name = name,
        tags = tags + ["ibazel_notify_changes"],
        **kwargs
    )

    if hygienic:
        _hygiene_test(
            name = name,
            tags = tags,
        )

def itest_task(name, tags = [], hygienic = True, **kwargs):
    _itest_task(
        name = name,
        tags = tags + ["ibazel_notify_changes"],
        **kwargs
    )

    if hygienic:
        _hygiene_test(
            name = name,
            tags = tags,
        )

def _hygiene_test(name, **kwargs):
    service_test(
        name = name + "_hygiene_test",
        services = [name],
        test = "@rules_itest//:exit0",
        **kwargs
    )

service_test = _service_test
