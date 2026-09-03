// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

#pragma once

#include <filesystem>
#include <optional>
#include <string>

#include "geniex.h"  // geniex_get_qairt_runtime_path
#include "types.h"

namespace geniex::qairt::runtime {

// NOTE: no `collect_bin_files()` helper here on purpose. Context-binary shards must
// come from genie_config.json's `ctx-bins` via the core's `modelConfigFromDirectory()`,
// never from a `*.bin` glob: bundles also ship CPU-side payloads as `.bin`.

inline std::optional<std::string> find_optional_file(const std::filesystem::path& dir, const char* filename) {
    const auto file_path = dir / filename;
    if (std::filesystem::exists(file_path)) {
        return file_path.string();
    }
    return std::nullopt;
}

// Returns the QnnRuntimeConfig every model in this plugin is created with.
//
// A runtime path set through geniex_set_qairt_runtime_path() is handed down as-is:
// the plugin owns resolution -- both accepted directory layouts, the GENIEX_QNN_LIB
// fallback, ADSP_LIBRARY_PATH and the Windows DLL search path -- so nothing here
// interprets or validates it. Unset leaves every field empty, which is how the plugin
// is told to use the QAIRT runtime it bundles.
inline QnnRuntimeConfig make_qnn_runtime_config() {
    QnnRuntimeConfig runtime_cfg{};

    const char* path = geniex_get_qairt_runtime_path();
    if (path != nullptr && *path != '\0') {
        GENIEX_LOG_INFO("Loading the QAIRT runtime from {} rather than the bundled one", path);
        runtime_cfg.htp_dir = path;
    }

    return runtime_cfg;
}

}  // namespace geniex::qairt::runtime
