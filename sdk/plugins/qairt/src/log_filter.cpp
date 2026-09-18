// Copyright (c) 2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

#include <cstdint>

#include "geniex.h"   // geniex_LogLevel, geniex_log_callback
#include "logging.h"  // SDK-side global sink: `geniex_log`

namespace geniex {
enum class LogLevel : uint32_t {
    Trace = 0,
    Debug = 1,
    Info  = 2,
    Warn  = 3,
    Error = 4,
};
using LogCallback = void (*)(LogLevel level, const char* message);
void geniex_set_log_callback(LogCallback cb);
}  // namespace geniex

namespace geniex {
namespace {

// Bridges the qairt core's own log callback (used for both plugin-internal
// GENIEX_LOG_* calls and QNN-originated "[QNN] ..." lines) into the SDK's
// `geniex_log` sink, converting the core's LogLevel to the SDK's
// geniex_LogLevel on the way. Once forwarded, the embedder's existing
// --log/GENIEX_LOG threshold decides what actually surfaces, same as every
// other SDK log line.

geniex_LogLevel toSdkLevel(LogLevel lvl) noexcept {
    switch (lvl) {
        case LogLevel::Trace:
            return GENIEX_LOG_LEVEL_TRACE;
        case LogLevel::Debug:
            return GENIEX_LOG_LEVEL_DEBUG;
        case LogLevel::Info:
            return GENIEX_LOG_LEVEL_INFO;
        case LogLevel::Warn:
            return GENIEX_LOG_LEVEL_WARN;
        case LogLevel::Error:
            return GENIEX_LOG_LEVEL_ERROR;
    }
    return GENIEX_LOG_LEVEL_INFO;
}

void filteringSink(LogLevel level, const char* message) {
    if (::geniex_log != nullptr && message != nullptr) {
        ::geniex_log(toSdkLevel(level), message);
    }
}

struct LogFilterInstaller {
    LogFilterInstaller() noexcept { geniex_set_log_callback(&filteringSink); }
};
const LogFilterInstaller g_installer{};

}  // namespace
}  // namespace geniex
