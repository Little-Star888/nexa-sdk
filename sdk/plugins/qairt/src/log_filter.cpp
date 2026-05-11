// Copyright (c) 2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

#include <atomic>
#include <cstdint>
#include <string_view>

#include "driver_status.h"
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

// The qairt core prefixes every QNN-originated log line with this marker
constexpr std::string_view kQnnPrefix = "[QNN]";

bool shouldSuppress(const char* msg) noexcept {
    if (msg == nullptr) return false;
    return std::string_view(msg).find(kQnnPrefix) != std::string_view::npos;
}

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
    if (shouldSuppress(message)) {
        // Raw QNN text is never forwarded to end users. But we still want to know
        // *that* a QNN error happened, so plugin-level code can give the user a
        // clean, actionable hint (most commonly: update your NPU driver).
        if (level == LogLevel::Error) {
            qairt::mark_qnn_error_seen();
        }
        return;
    }
    if (::geniex_log != nullptr && message != nullptr) {
        ::geniex_log(toSdkLevel(level), message);
    }
}

// Driver-status latch. Atomic so it's safe if QNN logs from worker threads.
std::atomic<bool>& qnnErrorLatch() noexcept {
    static std::atomic<bool> latch{false};
    return latch;
}

struct LogFilterInstaller {
    LogFilterInstaller() noexcept { geniex_set_log_callback(&filteringSink); }
};
const LogFilterInstaller g_installer{};

}  // namespace

namespace qairt {

void reset_qnn_error_flag() noexcept { qnnErrorLatch().store(false, std::memory_order_relaxed); }

bool qnn_error_seen() noexcept { return qnnErrorLatch().load(std::memory_order_relaxed); }

void mark_qnn_error_seen() noexcept { qnnErrorLatch().store(true, std::memory_order_relaxed); }

}  // namespace qairt

}  // namespace geniex
