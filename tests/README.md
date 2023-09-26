Try the following:
- `bazel test --test_output=streamed //e2e:e2e_test` - runs the test normally
- `brew install ibazel`
- `ibazel test //e2e:e2e_test` - this will rerun the test from scratch if anything changes
- `ibazel test //e2e:e2e_test_interactive` - this will run the test in interactive watch mode. If there is a change, only the test is restarted, the services stay running. This lets you iterate faster. In the future, we should detect which services need to be restarted as well.
