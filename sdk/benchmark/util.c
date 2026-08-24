// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

/* Error exits, file/string helpers, on-disk model size. */

#include <stdlib.h>
#include <string.h>

#include "bench.h"

void die(int32_t code, const char* what) {
    const char* msg = geniex_get_error_message((geniex_ErrorCode)code);
    fprintf(stderr, "ERROR: %s: %s (code=%d)\n", what, msg ? msg : "?", code);
    exit(1);
}

void check(int32_t code, const char* what) {
    if (code != GENIEX_SUCCESS) {
        die(code, what);
    }
}

char* rstrip(char* s) {
    size_t n = strlen(s);
    while (n > 0 && (s[n - 1] == '\n' || s[n - 1] == '\r' || s[n - 1] == ' ' || s[n - 1] == '\t')) {
        s[--n] = '\0';
    }
    return s;
}

char* slurp(const char* path) {
    FILE* f = fopen(path, "rb");
    if (!f) {
        fprintf(stderr, "ERROR: cannot open %s\n", path);
        exit(1);
    }
    fseek(f, 0, SEEK_END);
    long sz = ftell(f);
    fseek(f, 0, SEEK_SET);
    char* buf = (char*)malloc((size_t)sz + 1);
    if (!buf) {
        fclose(f);
        fprintf(stderr, "ERROR: oom slurping %s\n", path);
        exit(1);
    }
    if (fread(buf, 1, (size_t)sz, f) != (size_t)sz) {
        fclose(f);
        free(buf);
        fprintf(stderr, "ERROR: short read on %s\n", path);
        exit(1);
    }
    fclose(f);
    buf[sz] = '\0';
    return buf;
}

int64_t compute_model_size(const char* path) {
    struct stat st;
    if (stat(path, &st) != 0) return 0;
    if (S_ISREG(st.st_mode)) return (int64_t)st.st_size;
    if (!S_ISDIR(st.st_mode)) return 0;

    int64_t total = 0;
    size_t  plen  = strlen(path);
#ifdef _WIN32
    char pattern[1024];
    snprintf(pattern, sizeof(pattern), "%s\\*", path);
    WIN32_FIND_DATAA ffd;
    HANDLE           h = FindFirstFileA(pattern, &ffd);
    if (h == INVALID_HANDLE_VALUE) return 0;
    do {
        const char* name = ffd.cFileName;
        if (strcmp(name, ".") == 0 || strcmp(name, "..") == 0) continue;
        size_t need = plen + 1 + strlen(name) + 1;
        char*  cand = (char*)malloc(need);
        if (!cand) {
            FindClose(h);
            return total;
        }
        snprintf(cand, need, "%s/%s", path, name);
        total += compute_model_size(cand);
        free(cand);
    } while (FindNextFileA(h, &ffd));
    FindClose(h);
#else
    DIR* d = opendir(path);
    if (!d) return 0;
    struct dirent* e;
    while ((e = readdir(d)) != NULL) {
        if (strcmp(e->d_name, ".") == 0 || strcmp(e->d_name, "..") == 0) continue;
        size_t need = plen + 1 + strlen(e->d_name) + 1;
        char*  cand = (char*)malloc(need);
        if (!cand) {
            closedir(d);
            return total;
        }
        snprintf(cand, need, "%s/%s", path, e->d_name);
        total += compute_model_size(cand);
        free(cand);
    }
    closedir(d);
#endif
    return total;
}
