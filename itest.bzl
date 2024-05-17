""" Rules for running services in integration tests. """

load(
    "//private:itest.bzl",
    _itest_service = "itest_service",
    _itest_service_group = "itest_service_group",
    _itest_task = "itest_task",
    _service_test = "service_test",
)

def itest_service(name, tags = [], **kwargs):
    _itest_service(
        name = name,
        tags = tags + ["ibazel_notify_changes"],
        **kwargs
    )
    _hygiene_test(
        name = name,
        tags = tags,
    )

def itest_service_group(name, tags = [], **kwargs):
    _itest_service_group(
        name = name,
        tags = tags + ["ibazel_notify_changes"],
        **kwargs
    )
    _hygiene_test(
        name = name,
        tags = tags,
    )

def itest_task(name, tags = [], **kwargs):
    _itest_task(
        name = name,
        tags = tags + ["ibazel_notify_changes"],
        **kwargs
    )
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
