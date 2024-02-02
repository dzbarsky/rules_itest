""" This module defines a rule for bringing up mysqld in a hermetic way with pre-cached migrations. """

def _mysql_impl(ctx):
    mysqld = ctx.executable.mysqld

    mysql_dir = ctx.actions.declare_directory(ctx.attr.name + "_mysql")

    init_inputs = [ctx.file._mysql_conf]
    init_command = """
mkdir -p {mysql_data_dir} &&
# TODO(zbarsky): This leaves the root user passwordless. Simple enough, but we can fix up the password if we care.
exec {mysqld} --defaults-file={mysql_conf} --datadir="$(pwd)"/{mysql_data_dir} --initialize-insecure""".format(
        mysql_data_dir = mysql_dir.path + "/data",
        mysqld = mysqld.path,
        mysql_conf = ctx.file._mysql_conf.path,
    )

    progress_message = "Initializing DB"

    if ctx.file.init_sql:
        init_inputs.append(ctx.file.init_sql)
        init_command += " --init-file=$(pwd)/" + ctx.file.init_sql.path
        progress_message += " with migrations"

    ctx.actions.run_shell(
        command = init_command,
        progress_message = progress_message,
        inputs = init_inputs,
        outputs = [mysql_dir],
        tools = [mysqld],
    )

    executable = ctx.actions.declare_file(ctx.attr.name)

    ctx.actions.write(
        output = executable,
        content = """#!/bin/sh
set -eux
MYSQL_DIR="$TMPDIR/mysql"
mkdir -p "$MYSQL_DIR"
cp -r -L {mysql_data_dir} "$MYSQL_DIR"
chmod -R +w "$MYSQL_DIR"
exec {mysqld} --defaults-file={mysql_conf} --datadir="$MYSQL_DIR/data" $@ """.format(
            mysql_conf = ctx.file._mysql_conf.path,
            mysql_data_dir = mysql_dir.short_path + "/data",
            mysqld = mysqld.short_path,
        ),
        is_executable = True,
    )

    files = [executable, ctx.file._mysql_conf, mysql_dir]

    runfiles = ctx.runfiles(files = files).merge(
        ctx.attr.mysqld[DefaultInfo].default_runfiles,
    )

    return [
        DefaultInfo(
            executable = executable,
            files = depset(files),
            runfiles = runfiles,
        ),
    ]

mysql_impl = rule(
    implementation = _mysql_impl,
    attrs = {
        "init_sql": attr.label(
            default = None,
            allow_single_file = True,
        ),
        "mysqld": attr.label(
            executable = True,
            cfg = "exec",
        ),
        "_mysql_conf": attr.label(
            default = ":my.cnf",
            allow_single_file = True,
        ),
    },
    executable = True,
)
