load("@rules_shell//shell:sh_test.bzl", "sh_test")

def must_fail(name, test, **kwargs):
    sh_test(
        name = name,
        srcs = ["//:not.sh"],
        args = ["$(location :%s)" % test],
        data = [test],
        **kwargs,
    )
