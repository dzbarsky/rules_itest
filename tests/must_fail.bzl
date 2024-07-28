def must_fail(name, test):
    native.sh_test(
        name = name,
        srcs = ["//:not.sh"],
        args = ["$(location :%s)" % test],
        data = [test],
    )
