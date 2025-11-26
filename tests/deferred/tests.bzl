load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _failure_testing_test(ctx):
    """Test to verify that an analysis test may verify a rule fails with fail()."""
    env = analysistest.begin(ctx)

    asserts.expect_failure(env, "Non-deferred itest_service cannot depend on deferred itest_service: @@//deferred:non_deferred_depends_on_deferred_should_fail depends on @@//deferred:deferred_task")

    return analysistest.end(env)

failure_testing_test = analysistest.make(
    _failure_testing_test,
    expect_failure = True,
)

def tests():
    failure_testing_test(
        name = "test_non_deferred_depends_on_deferred_should_fail",
        target_under_test = ":non_deferred_depends_on_deferred_should_fail",
    )
