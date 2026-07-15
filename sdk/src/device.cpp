// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

// Single source of truth for the user-facing device alias table
// (cpu / gpu / npu / hybrid → concrete device_id + n_gpu_layers).
// Language bindings (Go CLI, Python, Android/JNI) call through to this.

#include <algorithm>
#include <cctype>
#include <cstdlib>
#include <string>

#include "geniex.h"
#include "logging.h"

#if defined(_WIN32)
#define portable_strdup _strdup
#else
#define portable_strdup strdup
#endif

namespace {

constexpr const char* kPluginQairt = "qairt";

constexpr const char* kAliasCPU    = "cpu";
constexpr const char* kAliasGPU    = "gpu";
constexpr const char* kAliasNPU    = "npu";
constexpr const char* kAliasHybrid = "hybrid";
constexpr const char* kAliasAuto   = "auto";

constexpr const char* kDeviceHTP0      = "HTP0";
constexpr const char* kDeviceGPUOpenCL = "GPUOpenCL";
constexpr const char* kDeviceQairtNPU  = "NPU";

std::string to_lower(const char* s) {
    if (!s) return {};
    std::string out(s);
    std::transform(
        out.begin(), out.end(), out.begin(), [](unsigned char c) { return static_cast<char>(std::tolower(c)); });
    return out;
}

std::string to_lower_trim(const char* s) {
    std::string out   = to_lower(s);
    size_t      start = 0;
    while (start < out.size() && std::isspace(static_cast<unsigned char>(out[start]))) ++start;
    size_t end = out.size();
    while (end > start && std::isspace(static_cast<unsigned char>(out[end - 1]))) --end;
    return out.substr(start, end - start);
}

bool is_known_alias(const std::string& s) {
    return s == kAliasCPU || s == kAliasGPU || s == kAliasNPU || s == kAliasHybrid;
}

}  // namespace

int32_t geniex_resolve_device(const geniex_ResolveDeviceInput* input, geniex_ResolveDeviceOutput* output) {
    if (!input || !output) {
        GENIEX_LOG_ERROR("geniex_resolve_device: input/output is null");
        return GENIEX_ERROR_COMMON_INVALID_INPUT;
    }

    // Initialise output so partial failures leave a sane state.
    output->device_id = nullptr;
    output->ngl       = input->ngl_default;
    output->warning   = nullptr;

    if (!input->plugin_id) {
        GENIEX_LOG_ERROR("geniex_resolve_device: plugin_id is null");
        return GENIEX_ERROR_COMMON_INVALID_INPUT;
    }

    const std::string plugin = input->plugin_id;
    std::string       alias  = to_lower_trim(input->mode);

    if (!alias.empty() && alias != kAliasAuto && !is_known_alias(alias)) {
        GENIEX_LOG_ERROR("geniex_resolve_device: invalid device mode '{}'", alias);
        return GENIEX_ERROR_COMMON_INVALID_DEVICE;
    }

    // Empty / "auto" → plugin default. Both qairt and llama_cpp default to
    // the pinned-NPU path.
    if (alias.empty() || alias == kAliasAuto) {
        alias = kAliasNPU;
    }

    // QAIRT is NPU-only and rejects any non-zero n_gpu_layers, so force
    // ngl to 0. Non-npu aliases are coerced with a warning, not an error.
    if (plugin == kPluginQairt) {
        if (alias != kAliasNPU) {
            std::string msg =
                "qairt plugin only supports NPU inference; ignoring device='" + alias + "' and running on NPU";
            output->warning = portable_strdup(msg.c_str());
        }
        output->device_id = portable_strdup(kDeviceQairtNPU);
        output->ngl       = 0;
        return GENIEX_SUCCESS;
    }

    // llama_cpp: ngl passes through unchanged (-1 means "all layers" to
    // llama.cpp). Only cpu forces it to 0.
    if (alias == kAliasCPU) {
        output->ngl = 0;
    } else if (alias == kAliasGPU) {
        output->device_id = portable_strdup(kDeviceGPUOpenCL);
    } else if (alias == kAliasNPU) {
        output->device_id = portable_strdup(kDeviceHTP0);
    }
    return GENIEX_SUCCESS;
}
