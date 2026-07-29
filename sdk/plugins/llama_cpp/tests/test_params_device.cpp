// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

// Pins the (device_id, n_gpu_layers) -> Device mapping that decides which
// backend the mtmd/clip vision projector loads onto in LlamaVlm::create.
//
// Why this exists: use_gpu has held three different expressions
// (n_gpu_layers > 0, then != 0, now device == Device::GPU). The != 0 form
// regressed #1178 -- npu and hybrid both carry the default ngl = -1, so clip
// selected the Adreno OpenCL backend during an NPU run and the shader
// compiler aborted at model load. Nothing pinned the mapping, so it went
// unnoticed.
//
// This is deliberately a *host* unit test, not an on-device one. The crash is
// only fatal where the Adreno OpenCL compiler is broken (QCS9075, #1250); on
// parts where OpenCL works, an end-to-end VLM run passes even with the wrong
// backend selected. Only asserting the mapping catches it everywhere.
//
// Rows mirror the alias table in sdk/src/device.cpp -- keep the two in sync.

#include <cstdio>

#include "params.h"

namespace {

struct Row {
    const char* label;      // alias or raw device_map id
    const char* device_id;  // as resolved by geniex_resolve_device
    int         ngl;        // as resolved by geniex_resolve_device
    bool        want_gpu;   // expected mparams.use_gpu in LlamaVlm::create
};

// geniex_resolve_device forces ngl = 0 for cpu and passes the caller default
// (-1 = all layers) through for gpu / npu / hybrid.
constexpr Row kRows[] = {
    // The four documented compute-unit aliases.
    {"cpu", nullptr, 0, false},
    {"gpu", "GPUOpenCL", -1, true},
    {"npu", "HTP0", -1, false},
    {"hybrid", "", -1, false},
    // Raw ids reach the plugin unresolved via device_map="<plugin>:<id>"
    // (bindings/python/geniex/auto.py). An unrecognised device and a
    // non-pinned HTP index must not instantiate OpenCL either.
    {"llama_cpp:HTP2", "HTP2", -1, false},
    {"llama_cpp:NOSUCHDEV", "NOSUCHDEV", -1, false},
    // ngl == 0 means pure CPU and outranks any device_id.
    {"gpu --ngl 0", "GPUOpenCL", 0, false},
};

}  // namespace

int main() {
    int failures = 0;

    for (const Row& row : kRows) {
        const geniex::Device device = geniex::classify_device(row.device_id, row.ngl);
        // The expression guarded in LlamaVlm::create.
        const bool use_gpu = device == geniex::Device::GPU;

        if (use_gpu != row.want_gpu) {
            std::printf("FAIL %-22s device_id=%-12s ngl=%3d -> use_gpu=%s, want %s\n",
                row.label,
                row.device_id ? row.device_id : "(null)",
                row.ngl,
                use_gpu ? "true" : "false",
                row.want_gpu ? "true" : "false");
            ++failures;
        } else {
            std::printf("ok   %-22s device_id=%-12s ngl=%3d -> use_gpu=%s\n",
                row.label,
                row.device_id ? row.device_id : "(null)",
                row.ngl,
                use_gpu ? "true" : "false");
        }
    }

    // Regression witness. The pre-#1178 expression was `n_gpu_layers != 0`.
    // Assert it still disagrees with the resolved-device answer, so that
    // "simplifying" use_gpu back to n_gpu_layers cannot look harmless: the
    // rows below are exactly the ones that crashed.
    int divergences = 0;
    for (const Row& row : kRows) {
        const bool ngl_based = row.ngl != 0;
        if (ngl_based != row.want_gpu) {
            std::printf("note n_gpu_layers != 0 is wrong for %-22s (would give use_gpu=%s)\n",
                row.label,
                ngl_based ? "true" : "false");
            ++divergences;
        }
    }
    if (divergences == 0) {
        std::printf("FAIL regression witness: n_gpu_layers != 0 no longer diverges; #1178 case lost\n");
        ++failures;
    }

    std::printf(
        "%d/%zu rows passed\n", (int)(sizeof(kRows) / sizeof(kRows[0])) - failures, sizeof(kRows) / sizeof(kRows[0]));
    return failures == 0 ? 0 : 1;
}
