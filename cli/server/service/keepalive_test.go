// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package service

import (
	"testing"

	geniex_sdk "github.com/qualcomm/GenieX/bindings/go"
)

// ResolveModelParam receives already-resolved knobs (the handler prefills unset
// request fields from the server defaults), so these tests pass the final
// values directly.

// TestResolveModelParam_PassesLlamaCppValuesThrough verifies that nctx / ngl are
// forwarded verbatim for llama_cpp and the compute alias resolves to a device.
func TestResolveModelParam_PassesLlamaCppValuesThrough(t *testing.T) {
	got, err := ResolveModelParam(geniex_sdk.RuntimeLlamaCpp, "some-model", 2048, 10, "gpu")
	if err != nil {
		t.Fatalf("ResolveModelParam: %v", err)
	}
	if got.NCtx != 2048 {
		t.Errorf("NCtx = %d, want 2048", got.NCtx)
	}
	if got.NGpuLayers != 10 {
		t.Errorf("NGpuLayers = %d, want 10", got.NGpuLayers)
	}
}

// TestResolveModelParam_NpuAliasResolvesDevice verifies the npu alias pins HTP0
// and passes ngl through (-1 = all layers).
func TestResolveModelParam_NpuAliasResolvesDevice(t *testing.T) {
	got, err := ResolveModelParam(geniex_sdk.RuntimeLlamaCpp, "some-model", 4096, -1, "npu")
	if err != nil {
		t.Fatalf("ResolveModelParam: %v", err)
	}
	if got.DeviceID != "HTP0" {
		t.Errorf("DeviceID = %q, want HTP0", got.DeviceID)
	}
	if got.NGpuLayers != -1 {
		t.Errorf("NGpuLayers = %d, want -1 (all layers)", got.NGpuLayers)
	}
}

// TestResolveModelParam_CpuAliasZeroesGpuLayers verifies ngl 0 (pure CPU) is a
// valid value that survives resolution.
func TestResolveModelParam_CpuAliasZeroesGpuLayers(t *testing.T) {
	got, err := ResolveModelParam(geniex_sdk.RuntimeLlamaCpp, "some-model", 4096, 0, "cpu")
	if err != nil {
		t.Fatalf("ResolveModelParam: %v", err)
	}
	if got.NGpuLayers != 0 {
		t.Errorf("NGpuLayers = %d, want 0 (pure CPU)", got.NGpuLayers)
	}
}

// TestResolveModelParam_NonLlamaCppZeroesNCtx verifies that for non-llama_cpp
// runtimes NCtx is zeroed so the plugin's param-guard is not tripped, even when
// the caller passes a non-zero value.
func TestResolveModelParam_NonLlamaCppZeroesNCtx(t *testing.T) {
	got, err := ResolveModelParam(geniex_sdk.RuntimeQairt, "some-model", 8192, 42, "")
	if err != nil {
		t.Fatalf("ResolveModelParam: %v", err)
	}
	if got.NCtx != 0 {
		t.Errorf("NCtx = %d, want 0 for non-llama_cpp", got.NCtx)
	}
	if got.NGpuLayers != 0 {
		t.Errorf("NGpuLayers = %d, want 0 (SDK zeroes ngl for qairt)", got.NGpuLayers)
	}
}
