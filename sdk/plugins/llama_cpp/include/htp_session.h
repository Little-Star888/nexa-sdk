// Copyright (c) 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

#pragma once

namespace geniex {

// HTP session lifecycle helpers shared by all llama.cpp plugin classes.
//
// llama.cpp's Hexagon backend opens FastRPC channels to the CDSP at registry
// construction time (ggml_hexagon_registry). Those channels persist for the
// life of the process, which means a QAIRT plugin spun up after llama.cpp
// tries to open its own dspqueue on the same CDSP domain and collides with
// llama.cpp's still-open libggml-htp-vN.so / libdspqueue_rpc_skel.so handles —
// QAIRT reports "Failed to create device: 1002" or 1007.
//
// Handoff to another plugin closes those channels via
// ggml_backend_hexagon_release_sessions (exposed by our llama.cpp patch).
// Before the next llama.cpp load we call ggml_backend_hexagon_reacquire_sessions
// so the cached device pointers in ggml-backend-reg have live sessions again.
//
// SessionGuard tracks live HTP users with a shared refcount but no longer
// releases on destruction — each release/reacquire cycle leaves the DSP-side
// dspqueue in a state that fails `dspqueue_read` (0x0d) after enough churn,
// so the actual release is deferred to release_sessions_if_idle(), invoked
// by the SDK only when a foreign plugin is about to load (Plugin::on_foreign_plugin_load).
namespace htp {

// Recreate any HTP sessions that were released by a prior handoff. No-op if
// the HTP backend is absent or sessions are already live. Safe to call on
// every llama.cpp load.
void reacquire_before_load();

// Returns true iff the ggml backend registry has a backend named "HTP".
// When true, llama.cpp has live FastRPC channels to CDSP that need to be
// torn down before a QAIRT plugin can take over.
bool htp_backend_present();

// Close all HTP sessions iff no SessionGuard is currently holding a
// reference. Called from LlamaPlugin::on_foreign_plugin_load when another
// plugin (QAIRT) is about to run, so its own HTP init isn't poisoned. No-op
// if the HTP backend is absent, sessions are already released, or any
// llama.cpp handle is still using HTP (the foreign plugin will collide, but
// that's caller error).
void release_sessions_if_idle();

class SessionGuard {
   public:
    SessionGuard() = default;
    ~SessionGuard() { release_ref(); }

    SessionGuard(const SessionGuard&)            = delete;
    SessionGuard& operator=(const SessionGuard&) = delete;

    // Call after deciding uses_htp (post model-load so we know the real
    // device selection). Safe to call multiple times — only the first flip
    // from false→true increments the shared refcount.
    void mark_htp();

    bool uses_htp() const { return uses_htp_; }

   private:
    // Decrement the shared refcount. Does NOT close sessions — release is
    // driven by release_sessions_if_idle() so sequential llama.cpp loads
    // keep the same HTP sessions warm.
    void release_ref();

    bool uses_htp_ = false;
};

}  // namespace htp
}  // namespace geniex
