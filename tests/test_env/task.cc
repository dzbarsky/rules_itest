#include <stdio.h>
#include <stdlib.h>
#include <string.h>

int main() {
    const char *want = "ITEST_ENV_VAR_VALUE";
    const char *got = getenv("ITEST_ENV_VAR");
    if (got == NULL) {
        fprintf(stderr, "ITEST_ENV_VAR_VALUE was not passed\n");
        return 1;
    }

    return strcmp(want, got);
}
