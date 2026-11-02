/* Test-only single-attempt native master handshake and root GETATTR readiness. */
#include <stdio.h>
#include <unistd.h>
#include <sys/stat.h>
#include <lizardfs/lizardfs_c_api.h>
int main(int argc, char **argv) {
    if (argc != 2) return 2;
    struct liz_init_params params;
    liz_set_default_init_params(&params, argv[1], "9421", "owlmail-native-readiness");
    params.subfolder = "/";
    params.delayed_init = 0;
    params.verbose = 1;
    liz_t *instance = liz_init_with_params(&params);
    if (!instance) {
        fprintf(stderr, "native init failed host=%s error=%d (%s)\n", argv[1], liz_last_err(), liz_error_string(liz_last_err()));
        return 3;
    }
    liz_context_t *ctx = liz_create_user_context(0, 0, getpid(), 0);
    if (!ctx) { liz_destroy(instance); return 4; }
    liz_attr_reply_t attrs;
    int rc = liz_getattr(instance, ctx, 1, &attrs);
    int err = rc < 0 ? liz_last_err() : 0;
    int valid = rc == 0 && attrs.attr.st_ino == 1 && S_ISDIR(attrs.attr.st_mode);
    fprintf(stdout, "native_root_getattr rc=%d error=%d root_directory=%d\n", rc, err, valid);
    liz_destroy_context(ctx);
    liz_destroy(instance);
    return valid ? 0 : 5;
}
