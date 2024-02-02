load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

def mysql_repositories():
    for (arch, sha256) in [
        ("macos13-arm64", "6876ae26c25288ebdd914c7757cd355b1667eb5c8c83a6cd395dfaa3522af706"),
        ("macos13-x86_64", "2d1f06ab923c5283d0e25ed396d24cfc9053c5e992191e262468f9e3e2cd97bb"),
        ("linux-glibc2.28-aarch64", "9159bd3e1ad66dd2ca09f1f7cdaf5458596dccaa44ab0cd7e3cdb071e79a6f9b"),
        ("linux-glibc2.28-x86_64", "491efad3cfbe7230ff1c6aef8d2f3d529b193b1d709eecc8566632f3fca391fd"),
    ]:
        http_archive(
            name = "mysql_8_0_34_" + arch,
            build_file = "//mysql:BUILD.mysql",
            urls = ["https://cdn.mysql.com/Downloads/MySQL-8.0/mysql-8.0.34-" + arch + ".tar.gz"],
            sha256 = sha256,
            strip_prefix = "mysql-8.0.34-" + arch,
        )
