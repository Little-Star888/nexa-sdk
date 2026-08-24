// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

/*
 * Shared types and cross-module declarations for geniex-bench.
 *
 * Module layout:
 *   benchmark.c — main(), single-cell driver, matrix-file driver
 *   options.c   — usage text + argv parsing
 *   resolve.c   — model-manager ids, local VLM bundle probing, path anchoring
 *   run.c       — LLM / VLM / raw-logits run loops
 *   stats.c     — median / min / max / mean / stdev aggregation
 *   report.c    — stdout summary line, JSON + Markdown reports
 *   util.c      — error exits, file + string helpers, on-disk model size
 */

#ifndef GENIEX_BENCH_H
#define GENIEX_BENCH_H

#include <geniex.h>
#include <geniex_model.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdio.h>
#include <sys/stat.h>

#ifdef _WIN32
#include <windows.h>
/* The MSVC CRT ships struct stat / stat() but not the POSIX type macros. */
#ifndef S_ISDIR
#define S_ISDIR(m) (((m) & _S_IFMT) == _S_IFDIR)
#endif
#ifndef S_ISREG
#define S_ISREG(m) (((m) & _S_IFMT) == _S_IFREG)
#endif
#else
#include <dirent.h>
#endif

#define MAX_PATHS 16

typedef struct {
    const char* plugin;
    const char* device;
    const char* device_id;
    /* model_path is either an absolute filesystem path (treated as a local
     * model directory or .gguf file) or a model-manager id of the form
     * `org/repo[:quant]`. The model-id branch resolves via geniex_model_*
     * (pulling if missing) and the resulting paths are written back into
     * `mm_model_path` / `mm_mmproj` / `mm_tokenizer`. `mm_model_path` and
     * `mm_tokenizer` always shadow their `_path` siblings; `mm_mmproj` only
     * shadows `mmproj_path` when the user explicitly opted into VLM (via
     * --vlm or matrix col 8), so a passively-present mmproj in the bundle
     * cannot redirect a `-p N` LLM bench into the VLM run loop (#1090). */
    const char* model_path;
    const char* tokenizer_path;
    const char* mmproj_path;
    /* Heap-owned copies populated when the model is resolved through the
     * model manager; freed at the end of run_one_cell. */
    char*       mm_model_path;
    char*       mm_mmproj;
    char*       mm_tokenizer;
    char*       mm_draft_model;
    bool        force_vlm; /* run VLM path even without an mmproj (QAIRT bundles) */
    bool        mm_is_vlm; /* manager classified the resolved model as VLM (geniex_ModelType) */
    const char* image_paths[MAX_PATHS];
    int32_t     image_count;
    const char* audio_paths[MAX_PATHS];
    int32_t     audio_count;

    int32_t n_prompt;   /* LLM random-ids prefill length (llama-bench -p), used when prompt_buf is NULL */
    char*   prompt_buf; /* heap-owned text prompt loaded via --prompt-file; NULL = use random-ids.
                         * Split into multiple prompts on lines that are exactly "---". */
    int32_t max_new_tokens;
    float   temperature;
    int32_t seed;
    int32_t warmup;
    int32_t repeat;
    bool    reset_between_runs; /* true => geniex_llm_reset() before each run, freeing KV */
    bool    accuracy;           /* true => single run (warmup=0, repeat=1), print generated text */

    /* Prefill-only raw-logits mode (--logits): one forward pass over the prompt,
     * no decode loop. Bypasses the timing/warmup/repeat machinery entirely. */
    bool    logits_mode;
    bool    logits_last_only; /* default: every position; set to emit only the last token's row */
    int32_t logits_top_n;     /* per row, emit only the top-N (token_id, logit) pairs */
    int32_t n_ctx;
    int32_t n_threads;
    int32_t ngl_override; /* -1 = use resolved alias default; >=0 overrides */

    const char* spec_type;    /* speculative type(s), comma-separated (llama_cpp); NULL = disabled */
    const char* draft_model;  /* draft/MTP GGUF for draft-* spec types (NULL for ngram-*) */
    int32_t     draft_tokens; /* max draft tokens per step (0 = plugin default) */
    int32_t     draft_min;    /* min draft tokens per step (0 = llama.cpp default) */
    float       draft_p_min;  /* min greedy draft probability (0 = llama.cpp default) */

    const char* output_json;
    const char* output_md;
    const char* cell_id;

    /* Matrix mode: one process, one geniex_init, many cells */
    const char* matrix_file;
    const char* output_json_dir;

    /* Model-manager options. Apply to every cell whose `model_path` looks
     * like a model id rather than a filesystem path. */
    const char* mm_data_dir; /* cache root; NULL falls back to GENIEX_DATADIR / ~/.cache/geniex */
    const char* mm_chipset;  /* AI Hub chipset slug (e.g. "qualcomm-snapdragon-x-elite") */
    const char* mm_hub;      /* "auto" | "hf" | "aihub" | "modelscope" | "volces" — default auto */
} options_t;

typedef struct {
    int32_t     run_idx;
    bool        is_warmup;
    int64_t     ttft_us;
    int64_t     media_us; /* image/audio encoder time; 0 for text-only */
    int64_t     prompt_time_us;
    int64_t     decode_time_us;
    int64_t     prompt_tokens; /* text + media tokens */
    int64_t     gen_tokens;
    double      prefill_tps;
    double      decode_tps;
    const char* stop_reason; /* not freed; lifetime tied to SDK output */
    int32_t     status;      /* 0 ok */
    char        err[256];
} run_result_t;

typedef struct {
    double ttft_ms_med, ttft_ms_lo, ttft_ms_hi, ttft_ms_mean, ttft_ms_sd;
    double prefill_med, prefill_lo, prefill_hi, prefill_mean, prefill_sd;
    double decode_med, decode_lo, decode_hi, decode_mean, decode_sd;
    double gen_tokens_med;
    double prompt_tokens_med;
    double media_ms_med;
} agg_t;

/* ------------------------------- util.c ------------------------------- */

/* Print `what` plus the SDK message for `code` to stderr and exit(1). */
void die(int32_t code, const char* what);
/* die() unless `code` is GENIEX_SUCCESS. */
void check(int32_t code, const char* what);
/* Strip trailing CR/LF/whitespace in place and return the input pointer. */
char* rstrip(char* s);
/* Load a whole file into a heap buffer (caller frees); exits on any failure. */
char* slurp(const char* path);
/* Total bytes the model occupies on disk: st_size for a .gguf file, recursive
 * sum of regular children for a bundle directory. 0 on stat failure. */
int64_t compute_model_size(const char* path);

/* ------------------------------ options.c ----------------------------- */

/* Parse argv into `o`, applying defaults and mode-specific validation.
 * Exits with a usage message on bad input. */
void parse_args(int argc, char** argv, options_t* o);

/* ------------------------------ resolve.c ----------------------------- */

/* True when `s` looks like a filesystem path rather than a model-manager id. */
bool looks_like_path(const char* s);
/* True when the local QAIRT bundle at `model_path` declares vision support. */
bool local_bundle_is_vlm(const char* model_path);
/* If `path` is a directory, return a heap path to a regular file inside it
 * (the SDK derives the model dir via parent_path()). NULL for a plain file. */
char* resolve_local_anchor(const char* path);
/* Resolve model-manager id `id_in` to local paths, pulling if missing, and
 * point o->model_path / mmproj_path / tokenizer_path at them. 0 on success. */
int resolve_via_mm(options_t* o, const char* id_in);
/* Same for o->draft_model when it is a model id rather than a path. */
int resolve_draft_via_mm(options_t* o);
/* Best-effort geniex_model_deinit() when the manager was initialised. */
void mm_shutdown(void);

/* -------------------------------- run.c ------------------------------- */

/* Optional per-token sleep for the on_token-overhead study
 * (--token-callback-delay-us); 0 keeps the callback a no-op. */
void set_token_callback_delay_us(int us);
/* warmup + repeat generation runs; writes `o->repeat` measured results. */
void run_llm(const options_t* o, const char* device_id, int32_t ngl, run_result_t* out);
void run_vlm(const options_t* o, const char* device_id, int32_t ngl, run_result_t* out);
/* --logits: one prefill-only forward pass, writes its own JSON report. */
void run_logits(const options_t* o, const char* device_id, int32_t ngl);

/* ------------------------------- stats.c ------------------------------ */

/* Reduce `n` measured runs to median / min / max / mean / stdev. */
void aggregate(const run_result_t* runs, int n, agg_t* a);

/* ------------------------------ report.c ----------------------------- */

void print_summary(const options_t* o, const char* device_id, int32_t ngl, const agg_t* a);
void write_json(const options_t* o, const char* device_id, int32_t ngl, int64_t model_size_bytes,
    const run_result_t* runs, const agg_t* a);
void write_md_row(const options_t* o, int32_t ngl, int64_t model_size_bytes, const agg_t* a);
void write_logits_json(const options_t* o, const char* device_id, int32_t ngl, const geniex_LlmForwardLogitsInput* fin,
    const geniex_LlmForwardLogitsOutput* fout);

#endif /* GENIEX_BENCH_H */
