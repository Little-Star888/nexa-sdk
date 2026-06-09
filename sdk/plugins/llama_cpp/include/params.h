#pragma once

#include <optional>
#include <vector>

#include "common.h"        // common_params_sampling
#include "geniex.h"        // geniex_ModelConfig
#include "ggml-backend.h"  // ggml_backend_dev_t
#include "llama.h"         // llama_model_params, llama_context_params

namespace geniex {

// Coarse compute target after device alias resolution (sdk/src/device.cpp).
// Drives per-(platform, device) defaults across all the build_* mappers
// below; mirrors test-llama.cpp's compute_configs.py ComputeUnit. HTP covers
// both the pinned `npu` alias and the `hybrid` per-tensor scheduler — they
// share the same upstream tuning.
enum class Device { CPU, GPU, HTP };

std::optional<std::vector<ggml_backend_dev_t>> resolve_devices(const char* device_id);

// Reverse-classify a resolved device selection. Mirrors the alias table in
// sdk/src/device.cpp:
//   cpu    -> n_gpu_layers == 0                     -> CPU
//   gpu    -> device_id starts with "GPU"           -> GPU
//   npu    -> device_id starts with "HTP"           -> HTP
//   hybrid -> empty device_id, ngl == 999           -> HTP
Device             classify_device(const char* device_id, int n_gpu_layers);
geniex_ModelConfig build_model_config(const geniex_ModelConfig& config, int32_t n_ctx_default, Device device);

llama_model_params     build_model_params(const geniex_ModelConfig& config);
llama_context_params   build_context_params(const geniex_ModelConfig& config, Device device);
ggml_threadpool_params build_threadpool_params(int n_threads, Device device);
common_params_sampling build_sampling_params(const geniex_SamplerConfig* cfg);

}  // namespace geniex
