// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

/* Per-cell aggregation of the measured runs. */

#include <math.h>
#include <stdlib.h>
#include <string.h>

#include "bench.h"

static int cmp_double(const void* a, const void* b) {
    double da = *(const double*)a;
    double db = *(const double*)b;
    return (da > db) - (da < db);
}

/* Reduce `values` to median / min / max / mean / sample stdev. Sorts a private
 * copy, so the caller's buffer is untouched. */
static stat_t summarize(const double* values, int32_t n) {
    double* tmp = (double*)malloc(sizeof(double) * (size_t)n);
    if (!tmp) {
        fprintf(stderr, "ERROR: oom\n");
        exit(1);
    }
    memcpy(tmp, values, sizeof(double) * (size_t)n);
    qsort(tmp, (size_t)n, sizeof(double), cmp_double);

    stat_t st;
    st.lo  = tmp[0];
    st.hi  = tmp[n - 1];
    st.med = (n % 2 == 1) ? tmp[n / 2] : 0.5 * (tmp[n / 2 - 1] + tmp[n / 2]);

    double sum = 0.0;
    for (int32_t i = 0; i < n; ++i) sum += tmp[i];
    st.mean = sum / (double)n;

    st.sd = 0.0;
    if (n >= 2) {
        double sq = 0.0;
        for (int32_t i = 0; i < n; ++i) {
            double d = tmp[i] - st.mean;
            sq += d * d;
        }
        st.sd = sqrt(sq / (double)(n - 1));
    }
    free(tmp);
    return st;
}

void aggregate(const run_result_t* runs, int32_t n, agg_t* a) {
    double* tmp = (double*)malloc(sizeof(double) * (size_t)n);
    if (!tmp) {
        fprintf(stderr, "ERROR: oom\n");
        exit(1);
    }
    for (int32_t i = 0; i < n; ++i) tmp[i] = (double)runs[i].ttft_us / 1000.0;
    a->ttft_ms = summarize(tmp, n);
    for (int32_t i = 0; i < n; ++i) tmp[i] = runs[i].prefill_tps;
    a->prefill_tps = summarize(tmp, n);
    for (int32_t i = 0; i < n; ++i) tmp[i] = runs[i].decode_tps;
    a->decode_tps = summarize(tmp, n);
    for (int32_t i = 0; i < n; ++i) tmp[i] = (double)runs[i].gen_tokens;
    a->gen_tokens_med = summarize(tmp, n).med;
    for (int32_t i = 0; i < n; ++i) tmp[i] = (double)runs[i].prompt_tokens;
    a->prompt_tokens_med = summarize(tmp, n).med;
    for (int32_t i = 0; i < n; ++i) tmp[i] = (double)runs[i].media_us / 1000.0;
    a->media_ms_med = summarize(tmp, n).med;
    free(tmp);
}
