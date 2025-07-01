#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <errno.h>
#include <sys/socket.h>
#include <sys/types.h>
#include <dlfcn.h>

#define LOG(fmt, ...) do { \
    char buf[512]; \
    int len = snprintf(buf, sizeof(buf), "[libreuseport] " fmt, ##__VA_ARGS__); \
    write(STDERR_FILENO, buf, len); \
} while (0)

// Log when the dylib is loaded
__attribute__((constructor))
static void init_notice() {
    LOG("dylib loaded\n");
}

// Helper for lazy dlsym
#define RESOLVE(name) \
    static typeof(name) *real_##name = NULL; \
    if (!real_##name) { \
        real_##name = (typeof(name) *)dlsym(RTLD_NEXT, #name); \
        if (!real_##name) { \
            LOG("dlsym failed for %s: %s\n", #name, dlerror()); \
            exit(1); \
        } \
    }

int socket(int domain, int type, int protocol) {
    RESOLVE(socket);
    int fd = real_socket(domain, type, protocol);
    LOG("socket(domain=%d, type=0x%x, protocol=%d) => %d\n", domain, type, protocol, fd);

    // Always try to set SO_REUSEPORT
    if (fd >= 0 && (type & SOCK_STREAM)) {
        int opt = 1;
        if (setsockopt(fd, SOL_SOCKET, SO_REUSEPORT, &opt, sizeof(opt)) != 0) {
            LOG("setsockopt(SO_REUSEPORT) failed on fd=%d: %s\n", fd, strerror(errno));
        } else {
            LOG("setsockopt(SO_REUSEPORT) succeeded on fd=%d\n", fd);
        }
    }
    return fd;
}

int bind(int fd, const struct sockaddr *addr, socklen_t len) {
    RESOLVE(bind);
    LOG("bind(fd=%d)\n", fd);
    return real_bind(fd, addr, len);
}

int connect(int fd, const struct sockaddr *addr, socklen_t len) {
    RESOLVE(connect);
    LOG("connect(fd=%d)\n", fd);
    return real_connect(fd, addr, len);
}

int listen(int fd, int backlog) {
    RESOLVE(listen);
    LOG("listen(fd=%d, backlog=%d)\n", fd, backlog);
    return real_listen(fd, backlog);
}

int accept(int fd, struct sockaddr *addr, socklen_t *len) {
    RESOLVE(accept);
    int client = real_accept(fd, addr, len);
    LOG("accept(fd=%d) => %d\n", fd, client);
    return client;
}

int socketpair(int domain, int type, int protocol, int sv[2]) {
    RESOLVE(socketpair);
    int res = real_socketpair(domain, type, protocol, sv);
    LOG("socketpair(domain=%d, type=0x%x, protocol=%d) => [%d,%d]\n",
        domain, type, protocol, sv[0], sv[1]);
    return res;
}

int pipe(int fds[2]) {
    RESOLVE(pipe);
    int res = real_pipe(fds);
    LOG("pipe() => [%d,%d]\n", fds[0], fds[1]);
    return res;
}

int pipe2(int fds[2], int flags) {
    RESOLVE(pipe2);
    int res = real_pipe2(fds, flags);
    LOG("pipe2(flags=0x%x) => [%d,%d]\n", flags, fds[0], fds[1]);
    return res;
}

int accept4(int fd, struct sockaddr *addr, socklen_t *len, int flags) {
    RESOLVE(accept4);
    int client = real_accept4(fd, addr, len, flags);
    LOG("accept4(fd=%d, flags=0x%x) => %d\n", fd, flags, client);
    return client;
}