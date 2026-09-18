// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

// Header-only power-mode alias table, included directly by the qairt and
// llama_cpp plugins. Not part of the public C ABI (no GENIEX_API symbol):
// the two plugins are separate shared libraries that don't link against
// each other or against libgeniex for this, so each resolves its own
// --power-mode string from this shared source instead of calling out to it.

#pragma once

#include <algorithm>
#include <cctype>
#include <string>
#include <utility>

#include "geniex.h"

namespace geniex::power_mode {

inline const std::pair<const char*, geniex_PowerMode> kAliases[] = {
    {"low_power_saver", GENIEX_POWER_MODE_LOW_POWER_SAVER},
    {"power_saver", GENIEX_POWER_MODE_POWER_SAVER},
    {"high_power_saver", GENIEX_POWER_MODE_HIGH_POWER_SAVER},
    {"low_balanced", GENIEX_POWER_MODE_LOW_BALANCED},
    {"balanced", GENIEX_POWER_MODE_BALANCED},
    {"high_performance", GENIEX_POWER_MODE_HIGH_PERFORMANCE},
    {"sustained_high_performance", GENIEX_POWER_MODE_SUSTAINED_HIGH_PERFORMANCE},
    {"burst", GENIEX_POWER_MODE_BURST},
};

// Resolves a user-facing power-mode alias into a geniex_PowerMode. NULL /
// "" / "default" -> burst. Matching is case-insensitive; surrounding
// whitespace is trimmed. Returns false if `mode` is a non-empty string
// that is not a documented alias.
inline bool resolve(const char* mode, geniex_PowerMode* out) {
    std::string alias(mode ? mode : "");
    size_t      start = 0;
    while (start < alias.size() && std::isspace(static_cast<unsigned char>(alias[start]))) ++start;
    size_t end = alias.size();
    while (end > start && std::isspace(static_cast<unsigned char>(alias[end - 1]))) --end;
    alias = alias.substr(start, end - start);
    std::transform(
        alias.begin(), alias.end(), alias.begin(), [](unsigned char c) { return static_cast<char>(std::tolower(c)); });

    if (alias.empty() || alias == "default") {
        *out = GENIEX_POWER_MODE_BURST;
        return true;
    }

    for (const auto& entry : kAliases) {
        if (alias == entry.first) {
            *out = entry.second;
            return true;
        }
    }
    return false;
}

}  // namespace geniex::power_mode
