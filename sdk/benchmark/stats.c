// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

/* Per-cell aggregation of the measured runs. */

#include <math.h>
#include <stdlib.h>

#include "bench.h"

static int cmp_double(const void* a, const void* b) {
    double da = *(const double*)a;
    double db = *(const double*)b;
    return (da > db) - (da < db);
}

/* Reduce `values` to median / min / max / mean / sample stdev, sorting it in
 * place. n=1 yields sd=0. Requires n >= 1: lo/hi index values[0] and
 * values[n-1] unconditionally. */
static stat_t summarize(double* values, int32_t n) {
    qsort(values, (size_t)n, sizeof(double), cmp_double);

    stat_t st;
    st.lo  = values[0];
    st.hi  = values[n - 1];
    st.med = (n % 2 == 1) ? values[n / 2] : 0.5 * (values[n / 2 - 1] + values[n / 2]);

    double sum = 0.0;
    for (int32_t i = 0; i < n; ++i) sum += values[i];
    st.mean = sum / (double)n;

    st.sd = 0.0;
    if (n >= 2) {
        double sq = 0.0;
        for (int32_t i = 0; i < n; ++i) {
            double d = values[i] - st.mean;
            sq += d * d;
        }
        st.sd = sqrt(sq / (double)(n - 1));
    }
    return st;
}

void aggregate_runs(const run_result_t* runs, int32_t n, agg_t* a) {
    /* parse_args validates --repetitions >= 1, so this only fires on a caller
     * bug — but summarize() would read out of bounds rather than say so. */
    if (n < 1) {
        fprintf(stderr, "ERROR: aggregate_runs: n=%d\n", n);
        exit(1);
    }
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
