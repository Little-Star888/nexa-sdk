// Copyright (c) 2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

#pragma once

#include <string>

#if defined(_WIN32)
#include <windows.h>
#endif

namespace geniex::qairt {

// The vision pipeline (geniex-proc) opens image files with std::filesystem and stb_image,
// both of which interpret narrow paths in the active ANSI code page rather than UTF-8. On
// Windows that breaks any non-ASCII path. Collapse the UTF-8 path to its 8.3 short name and
// re-encode it in that same ANSI code page, so those APIs open it correctly. No-op elsewhere,
// and falls back to the original path when no ANSI-representable short name is available (e.g.
// 8.3 disabled on the volume).
inline std::string to_loadable_path(const std::string& utf8_path) {
#if defined(_WIN32)
    int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8_path.c_str(), -1, nullptr, 0);
    if (wlen <= 0) return utf8_path;
    std::wstring wpath(wlen, L'\0');
    MultiByteToWideChar(CP_UTF8, 0, utf8_path.c_str(), -1, wpath.data(), wlen);

    DWORD slen = GetShortPathNameW(wpath.c_str(), nullptr, 0);
    if (slen == 0) return utf8_path;
    std::wstring wshort(slen, L'\0');
    if (GetShortPathNameW(wpath.c_str(), wshort.data(), slen) == 0) return utf8_path;

    // Encode in the active ANSI code page — the exact encoding the downstream narrow-path APIs
    // decode with. If any character has no ANSI representation, bail to the original path rather
    // than hand over a lossy substitution.
    int nlen = WideCharToMultiByte(CP_ACP, 0, wshort.c_str(), -1, nullptr, 0, nullptr, nullptr);
    if (nlen <= 0) return utf8_path;
    std::string narrow(nlen, '\0');
    BOOL        used_default = FALSE;
    WideCharToMultiByte(CP_ACP, 0, wshort.c_str(), -1, narrow.data(), nlen, nullptr, &used_default);
    if (used_default) return utf8_path;
    narrow.resize(nlen - 1);  // drop the terminating NUL from the count
    return narrow;
#else
    return utf8_path;
#endif
}

}  // namespace geniex::qairt
