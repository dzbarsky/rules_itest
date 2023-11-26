This repository contains a Bazel ruleset for running tests that depend on services.
For example, you may want to run integration tests that depend on having a DB, redis, etc. running.
Or you may be integration-testing first party binaries. Each test can declare the set of services that it expects,
and it is guaranteed to be provisioned with a fresh set of services that have passed healthchecks before the
test code is executed.

This framework also integrates with [bazel-watcher](https://github.com/bazelbuild/bazel-watcher) to provide incremental hot-reload capabilities.

Try the following:
- `cd examples`
- `bazel test --test_output=streamed //e2e:e2e_test` - runs the test normally
- `brew install ibazel`
- `ibazel test //e2e:e2e_test` - this will run the test in interactive watch mode. If the code changes, only the services that depend on the code are restarted (potentially none), and then the test is re-executed. This lets you iterate faster compared to starting services from scratch every time. Note that this requires a bit of care when writing the test, so it does not fail when executed multiple times.
