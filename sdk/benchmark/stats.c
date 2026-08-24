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

static void summarize(const double* values, int n, double* median, double* lo, double* hi) {
    double* tmp = (double*)malloc(sizeof(double) * (size_t)n);
    memcpy(tmp, values, sizeof(double) * (size_t)n);
    qsort(tmp, (size_t)n, sizeof(double), cmp_double);
    *lo     = tmp[0];
    *hi     = tmp[n - 1];
    *median = (n % 2 == 1) ? tmp[n / 2] : 0.5 * (tmp[n / 2 - 1] + tmp[n / 2]);
    free(tmp);
}

/* Same shape as summarize() but also yields mean / sample stdev. n=1 emits
 * stdev=0 (sample stdev with one observation is undefined; surface 0 to
 * match llama-bench's `123.4 ± 0.0` rendering). */
static void summarize_full(
    const double* values, int n, double* median, double* lo, double* hi, double* mean, double* sd) {
    summarize(values, n, median, lo, hi);
    double sum = 0.0;
    for (int i = 0; i < n; ++i) sum += values[i];
    *mean = sum / (double)n;
    if (n < 2) {
        *sd = 0.0;
        return;
    }
    double sq = 0.0;
    for (int i = 0; i < n; ++i) {
        double d = values[i] - *mean;
        sq += d * d;
    }
    *sd = sqrt(sq / (double)(n - 1));
}

void aggregate(const run_result_t* runs, int n, agg_t* a) {
    double* tmp = (double*)malloc(sizeof(double) * (size_t)n);
    if (!tmp) {
        fprintf(stderr, "ERROR: oom\n");
        exit(1);
    }
    for (int i = 0; i < n; ++i) {
        tmp[i] = (double)runs[i].ttft_us / 1000.0;
    }
    summarize_full(tmp, n, &a->ttft_ms_med, &a->ttft_ms_lo, &a->ttft_ms_hi, &a->ttft_ms_mean, &a->ttft_ms_sd);
    for (int i = 0; i < n; ++i) tmp[i] = runs[i].prefill_tps;
    summarize_full(tmp, n, &a->prefill_med, &a->prefill_lo, &a->prefill_hi, &a->prefill_mean, &a->prefill_sd);
    for (int i = 0; i < n; ++i) tmp[i] = runs[i].decode_tps;
    summarize_full(tmp, n, &a->decode_med, &a->decode_lo, &a->decode_hi, &a->decode_mean, &a->decode_sd);
    for (int i = 0; i < n; ++i) tmp[i] = (double)runs[i].gen_tokens;
    double med, lo, hi;
    summarize(tmp, n, &med, &lo, &hi);
    a->gen_tokens_med = med;
    for (int i = 0; i < n; ++i) tmp[i] = (double)runs[i].prompt_tokens;
    summarize(tmp, n, &med, &lo, &hi);
    a->prompt_tokens_med = med;
    for (int i = 0; i < n; ++i) tmp[i] = (double)runs[i].media_us / 1000.0;
    summarize(tmp, n, &med, &lo, &hi);
    a->media_ms_med = med;
    free(tmp);
}
