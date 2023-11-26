""" Rules for running services in integration tests. """

load(
    "//private:itest.bzl",
    _itest_service = "itest_service",
    _itest_service_group = "itest_service_group",
    _itest_task = "itest_task",
    _service_test = "service_test",
)

def itest_service(name, **kwargs):
    _itest_service(name = name, **kwargs)

    service_test(
        name = name + "_hygiene_test",
        services = [name],
        test = "@rules_itest//:exit0",
    )

def itest_service_group(**kwargs):
    _itest_service_group(**kwargs)

def itest_task(**kwargs):
    _itest_task(**kwargs)

def service_test(tags = [], **kwargs):
    _service_test(tags = tags + ["ibazel_notify_changes"], **kwargs)
