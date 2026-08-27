// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package geniex_sdk

import "testing"

// No ModelInit: the alias table is static, so no store is involved.
func TestResolveAlias(t *testing.T) {
	got, ok := ResolveAlias("qwen3")
	if !ok {
		t.Fatal("qwen3 is the SDK's alias fixture")
	}
	if want := "ggml-org/Qwen3-1.7B-GGUF:Q4_K_M"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	for _, alias := range []string{"", "no-such-alias-xyz-9999", "ggml-org/Qwen3-1.7B-GGUF"} {
		if got, ok := ResolveAlias(alias); ok {
			t.Errorf("ResolveAlias(%q): expected miss, got %q", alias, got)
		}
	}
}
