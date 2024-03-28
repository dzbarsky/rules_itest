# Bazel rules for integration testing and dev environments

This repository contains a Bazel ruleset for running services in a hermetic way. This capability can be used to provision a dev environment, or to run tests against the services.

For example, you may want to run integration tests that depend on having a DB, redis, etc. running. Or you may be integration-testing first party binaries.

With this ruleset, each test can declare the set of services that it expects, and it is guaranteed to be provisioned with a fresh set of services that have passed healthchecks before the test code is executed.

This ruleset also integrates with [ibazel](https://github.com/bazelbuild/bazel-watcher) to provide incremental hot-reload capabilities.

# Usage
See the documentation in the [docs folder](https://github.com/dzbarsky/rules_itest/blob/master/docs/itest.md).

Note that the implementation of per-service reload under ibazel works by injecting a cache-busting input, so it is disabled by default to keep tests cacheable. You can enable it with an extra CLI flag, like so:
`ibazel run @rules_itest//:enable_per_service_reload //path/to:target`

# Examples
Basic usage examples can be found under the [examples folder](https://github.com/dzbarsky/rules_itest/tree/master/examples).

The examples show how to run mysql, redis, dynamodb, as well as Golang and Node binaries.