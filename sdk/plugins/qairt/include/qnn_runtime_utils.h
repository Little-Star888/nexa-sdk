// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

#pragma once

#if defined(_WIN32)
#include <windows.h>
#endif

#include <cstdlib>
#include <filesystem>
#include <optional>
#include <stdexcept>
#include <string>
#include <vector>

#include "types.h"

namespace geniex::qairt::runtime {

// Reads an environment variable as a path; empty when unset. Takes both spellings of the
// name because Windows needs the wide-char API for Unicode paths to round-trip.
inline std::filesystem::path read_env_path(const char* name_utf8, const wchar_t* name_wide) {
#if defined(_WIN32)
    static_cast<void>(name_utf8);
    size_t required_size = 0;
    _wgetenv_s(&required_size, nullptr, 0, name_wide);
    if (required_size > 0) {
        std::vector<wchar_t> env_buffer(required_size);
        _wgetenv_s(&required_size, env_buffer.data(), required_size, name_wide);
        if (env_buffer[0] != L'\0') {
            return std::filesystem::path(env_buffer.data());
        }
    }
    return {};
#else   // _WIN32
    static_cast<void>(name_wide);
    if (const char* value = std::getenv(name_utf8)) {
        return std::filesystem::path(value);
    }
    return {};
#endif  // _WIN32
}

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

// QNN library names, the SDK's host-lib subfolder, and the OS PATH separator, by platform.
#if defined(_WIN32)
constexpr const char* kQnnBackendLib    = "QnnHtp.dll";
constexpr const char* kQnnSystemLib     = "QnnSystem.dll";
constexpr const char* kQnnExtensionsLib = "QnnHtpNetRunExtensions.dll";
constexpr const char* kHostLibTriple    = "aarch64-windows-msvc";
constexpr char        kPathSep          = ';';
#elif defined(__ANDROID__)
constexpr const char* kQnnBackendLib    = "libQnnHtp.so";
constexpr const char* kQnnSystemLib     = "libQnnSystem.so";
constexpr const char* kQnnExtensionsLib = "libQnnHtpNetRunExtensions.so";
constexpr const char* kHostLibTriple    = "aarch64-android";
constexpr char        kPathSep          = ':';
#else  // __linux__
constexpr const char* kQnnBackendLib    = "libQnnHtp.so";
constexpr const char* kQnnSystemLib     = "libQnnSystem.so";
constexpr const char* kQnnExtensionsLib = "libQnnHtpNetRunExtensions.so";
constexpr const char* kHostLibTriple    = "aarch64-oe-linux-gcc11.2";
constexpr char        kPathSep          = ':';
#endif

// Finds the directory holding the host QNN libraries under `root`, which may be either a
// flat folder or a QAIRT SDK install. Empty when neither layout matches.
inline std::filesystem::path locate_qnn_host_lib_dir(const std::filesystem::path& root) {
    namespace fs = std::filesystem;
    std::error_code ec;

    // Flat folder: libs directly inside root.
    if (fs::exists(root / kQnnBackendLib, ec)) return root;

    // QAIRT SDK: canonical per-platform triple.
    const fs::path triple_dir = root / "lib" / kHostLibTriple;
    if (fs::exists(triple_dir / kQnnBackendLib, ec)) return triple_dir;

    // Fallback for triples that vary across SDK versions (e.g. differing Linux gcc suffixes).
    const fs::path lib_dir = root / "lib";
    if (fs::is_directory(lib_dir, ec)) {
        for (const auto& entry : fs::directory_iterator(lib_dir, ec)) {
            if (entry.is_directory(ec) && fs::exists(entry.path() / kQnnBackendLib, ec)) {
                return entry.path();
            }
        }
    }
    return {};
}

// Builds an ADSP_LIBRARY_PATH covering every Hexagon skel folder in a QAIRT SDK, so QNN's
// loader can pick the one matching the on-device HTP arch. Empty when `root` ships no skels.
inline std::string collect_adsp_library_path(const std::filesystem::path& root) {
    namespace fs = std::filesystem;
    std::error_code          ec;
    std::vector<std::string> dirs;

    const fs::path lib_dir = root / "lib";
    if (fs::is_directory(lib_dir, ec)) {
        for (const auto& entry : fs::directory_iterator(lib_dir, ec)) {
            if (!entry.is_directory(ec)) continue;
            if (entry.path().filename().string().rfind("hexagon-", 0) != 0) continue;
            const fs::path unsigned_dir = entry.path() / "unsigned";
            dirs.push_back(fs::is_directory(unsigned_dir, ec) ? unsigned_dir.string() : entry.path().string());
        }
    }

    std::sort(dirs.begin(), dirs.end());
    std::string joined;
    for (const auto& d : dirs) {
        if (!joined.empty()) joined.push_back(kPathSep);
        joined += d;
    }
    return joined;
}

// Returns a QnnRuntimeConfig for the given model directory.
//
// GENIEX_QAIRT_LIB (or the CLI `--qairt-lib` flag) is an optional override; unset, the config
// stays empty and the plugin resolves its own bundled runtime. Set, we pin all three path
// fields, which the plugin then honors as-is -- translating an SDK root is our job because
// the plugin only understands the flat layout. Throws when set but unusable.
inline QnnRuntimeConfig make_qnn_runtime_config(const std::filesystem::path& model_dir) {
    namespace fs = std::filesystem;

    QnnRuntimeConfig runtime_cfg{};

    const fs::path qnn_lib_root = read_env_path("GENIEX_QAIRT_LIB", L"GENIEX_QAIRT_LIB");
    if (qnn_lib_root.empty()) {
        GENIEX_LOG_DEBUG("GENIEX_QAIRT_LIB unset; using the QAIRT runtime bundled with the plugin");
        static_cast<void>(model_dir);
        return runtime_cfg;
    }

    std::error_code ec;
    if (!fs::is_directory(qnn_lib_root, ec)) {
        throw std::runtime_error("GENIEX_QAIRT_LIB path is not a directory: " + qnn_lib_root.string() +
                                 "\nUnset it to use the QAIRT runtime bundled with the plugin.");
    }
    const fs::path host_dir = locate_qnn_host_lib_dir(qnn_lib_root);
    if (host_dir.empty()) {
        throw std::runtime_error("GENIEX_QAIRT_LIB does not contain " + std::string(kQnnBackendLib) +
                                 " (looked in the folder itself and lib/" + kHostLibTriple +
                                 "): " + qnn_lib_root.string() +
                                 "\nUnset it to use the QAIRT runtime bundled with the plugin.");
    }

    // A QAIRT SDK keeps skels apart from the host libs; a flat folder has them together.
    std::string adsp_path = collect_adsp_library_path(qnn_lib_root);
    if (adsp_path.empty()) adsp_path = host_dir.string();

    GENIEX_LOG_INFO("Overriding the bundled QAIRT runtime from GENIEX_QAIRT_LIB: {} (host libs: {})",
        qnn_lib_root.string(),
        host_dir.string());
    GENIEX_LOG_DEBUG("Setting ADSP_LIBRARY_PATH to {}", adsp_path);
#if defined(_WIN32)
    _putenv_s("ADSP_LIBRARY_PATH", adsp_path.c_str());
    SetDllDirectoryA(host_dir.string().c_str());
#else
    setenv("ADSP_LIBRARY_PATH", adsp_path.c_str(), 1);
#endif
    runtime_cfg.backend_path    = (host_dir / kQnnBackendLib).string();
    runtime_cfg.system_lib_path = (host_dir / kQnnSystemLib).string();
    runtime_cfg.extensions_path = (host_dir / kQnnExtensionsLib).string();

    static_cast<void>(model_dir);
    return runtime_cfg;
}

}  // namespace geniex::qairt::runtime
