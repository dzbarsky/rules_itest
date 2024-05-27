<!-- Generated with Stardoc: http://skydoc.bazel.build -->


# Rules for running services in integration tests.

This ruleset supports [ibazel](https://github.com/bazelbuild/bazel-watcher) when using `bazel run`.
As a UX optimization, the service manager is able to restart only the modified services, instead of all services,
when it receives the reload notification from ibazel. This capability depends on a cache-busting input, so it is hidden
behind an an extra CLI flag, like so:
```
.bazelrc

build:enable-reload --@rules_itest//:enable_per_service_reload
fetch:enable-reload --@rules_itest//:enable_per_service_reload
query:enable-reload --@rules_itest//:enable_per_service_reload
```

`ibazel run --config enable-reload //path/to:target`

In addition, if can set the `hot_reloadable` attribute on an `itest_service`, the service manager will
forward the ibazel hot-reload notification over stdin instead of restarting the service.


<a id="itest_service"></a>

## itest_service

<pre>
itest_service(<a href="#itest_service-name">name</a>, <a href="#itest_service-autoassign_port">autoassign_port</a>, <a href="#itest_service-data">data</a>, <a href="#itest_service-deps">deps</a>, <a href="#itest_service-env">env</a>, <a href="#itest_service-exe">exe</a>, <a href="#itest_service-health_check">health_check</a>, <a href="#itest_service-health_check_args">health_check_args</a>,
              <a href="#itest_service-health_check_interval">health_check_interval</a>, <a href="#itest_service-hot_reloadable">hot_reloadable</a>, <a href="#itest_service-http_health_check_address">http_health_check_address</a>, <a href="#itest_service-named_ports">named_ports</a>)
</pre>

An itest_service is a binary that is intended to run for the duration of the integration test. Examples include databases, HTTP/RPC servers, queue consumers, external service mocks, etc.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="itest_service-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="itest_service-autoassign_port"></a>autoassign_port |  If true, the service manager will pick a free port and assign it to the service.         The port will be interpolated into <code>$${PORT}</code> in the service's <code>http_health_check_address</code> and <code>args</code>.         It will also be exported under the target's fully qualified label in the service-port mapping.<br><br>        The assigned ports for all services are available for substitution in <code>http_health_check_address</code> and <code>args</code> (in case one service needs the address for another one.)         For example, the following substitution: <code>args = ["-client-addr", "127.0.0.1:$${@@//label/for:service}"]</code><br><br>        The service-port mapping is a JSON string -&gt; int map propagated through the <code>ASSIGNED_PORTS</code> env var.         For example, a port can be retrieved with the following JS code:         <code>JSON.parse(process.env["ASSIGNED_PORTS"])["@@//label/for:service"]</code>.<br><br>        Alternately, the env will also contain the location of a binary that can return the port, for contexts without a readily-accessible JSON parser.         For example, the following Bash command:         <code>PORT=$($GET_ASSIGNED_PORT_BIN @@//label/for:service)</code>   | Boolean | optional | <code>False</code> |
| <a id="itest_service-data"></a>data |  -   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |
| <a id="itest_service-deps"></a>deps |  Services/tasks that must be started before this service/task can be started. Can be <code>itest_service</code>, <code>itest_task</code>, or <code>itest_service_group</code>.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |
| <a id="itest_service-env"></a>env |  The service manager will merge these variables into the environment when spawning the underlying binary.   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional | <code>{}</code> |
| <a id="itest_service-exe"></a>exe |  The binary target to run.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |
| <a id="itest_service-health_check"></a>health_check |  If set, the service manager will execute this binary to check if the service came up in a healthy state.         This check will be retried until it exits with a 0 exit code. When used in conjunction with autoassigned ports, use         one of the methods described in <code>autoassign_port</code> to locate the service.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional | <code>None</code> |
| <a id="itest_service-health_check_args"></a>health_check_args |  Arguments to pass to the health_check binary. The various defined ports will be substituted prior to being given to the health_check binary.   | List of strings | optional | <code>[]</code> |
| <a id="itest_service-health_check_interval"></a>health_check_interval |  The duration between each health check. The syntax is based on common time duration with a number, followed by the time unit. For example, <code>200ms</code>, <code>1s</code>, <code>2m</code>, <code>3h</code>, <code>4d</code>.   | String | optional | <code>"200ms"</code> |
| <a id="itest_service-hot_reloadable"></a>hot_reloadable |  If set to True, the service manager will propagate ibazel's reload notificaiton over stdin instead of restarting the service.         See the ruleset docstring for more info on using ibazel   | Boolean | optional | <code>False</code> |
| <a id="itest_service-http_health_check_address"></a>http_health_check_address |  If set, the service manager will send an HTTP request to this address to check if the service came up in a healthy state.         This check will be retried until it returns a 200 HTTP code. When used in conjunction with autoassigned ports, <code>$${@@//label/for:service:port_name}</code> can be used in the address.         Example: <code>http_health_check_address = "http://127.0.0.1:$${@@//label/for:service:port_name}",</code>   | String | optional | <code>""</code> |
| <a id="itest_service-named_ports"></a>named_ports |  For each element of the list, the service manager will pick a free port and assign it to the service.         The port's fully-qualified name is the service's fully-qualified label and the port name, separated by a colon.         For example, a port assigned with <code>names_ports = ["http_port"]</code> will be assigned a fully-qualified name of <code>@@//label/for:service:http_port</code>.<br><br>        Named ports are accessible through the service-port mapping. For more details, see <code>autoassign_port</code>.   | List of strings | optional | <code>[]</code> |


<a id="itest_service_group"></a>

## itest_service_group

<pre>
itest_service_group(<a href="#itest_service_group-name">name</a>, <a href="#itest_service_group-services">services</a>)
</pre>

A service group is a collection of services/tasks.

It serves as a convenient way for a downstream target to depend on multiple services with a single label, without
forcing the services within the group to define a specific startup ordering with their `deps`.

It is also useful to bring up multiple services with a single `bazel run` command, which is useful for creating
dev environments.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="itest_service_group-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="itest_service_group-services"></a>services |  Services/tasks that comprise this group. Can be <code>itest_service</code>, <code>itest_task</code>, or <code>itest_service_group</code>.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |


<a id="itest_task"></a>

## itest_task

<pre>
itest_task(<a href="#itest_task-name">name</a>, <a href="#itest_task-data">data</a>, <a href="#itest_task-deps">deps</a>, <a href="#itest_task-env">env</a>, <a href="#itest_task-exe">exe</a>)
</pre>

A task is a one-shot (not long-running binary) that is intended to be executed as part of the itest scenario creation.
Examples include: filesystem setup, dynamic config file generation (especially if it depends on ports), DB migrations or seed data creation

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="itest_task-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="itest_task-data"></a>data |  -   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |
| <a id="itest_task-deps"></a>deps |  Services/tasks that must be started before this service/task can be started. Can be <code>itest_service</code>, <code>itest_task</code>, or <code>itest_service_group</code>.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |
| <a id="itest_task-env"></a>env |  The service manager will merge these variables into the environment when spawning the underlying binary.   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional | <code>{}</code> |
| <a id="itest_task-exe"></a>exe |  The binary target to run.   | <a href="https://bazel.build/concepts/labels">Label</a> | required |  |


<a id="service_test"></a>

## service_test

<pre>
service_test(<a href="#service_test-name">name</a>, <a href="#service_test-data">data</a>, <a href="#service_test-env">env</a>, <a href="#service_test-services">services</a>, <a href="#service_test-test">test</a>)
</pre>

Brings up a set of services/tasks and runs a test target against them.

This can be used to customize which services a particular test needs while being able to bring them up in an easy and consistent way.

Example usage:
```
go_test(
    name = "_example_test_no_services",
    srcs = [..],
    tags = ["manual"],
)

service_test(
    name = "example_test",
    test = ":_example_test_no_services",
    services = [
        "//services/mysql",
        ...
    ],
)
```

Typically this would be wrapped into a macro.

**ATTRIBUTES**


| Name  | Description | Type | Mandatory | Default |
| :------------- | :------------- | :------------- | :------------- | :------------- |
| <a id="service_test-name"></a>name |  A unique name for this target.   | <a href="https://bazel.build/concepts/labels#target-names">Name</a> | required |  |
| <a id="service_test-data"></a>data |  -   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |
| <a id="service_test-env"></a>env |  The service manager will merge these variables into the environment when spawning the underlying binary.   | <a href="https://bazel.build/rules/lib/dict">Dictionary: String -> String</a> | optional | <code>{}</code> |
| <a id="service_test-services"></a>services |  Services/tasks that comprise this group. Can be <code>itest_service</code>, <code>itest_task</code>, or <code>itest_service_group</code>.   | <a href="https://bazel.build/concepts/labels">List of labels</a> | optional | <code>[]</code> |
| <a id="service_test-test"></a>test |  The underlying test target to execute once the services have been brought up and healthchecked.   | <a href="https://bazel.build/concepts/labels">Label</a> | optional | <code>None</code> |


