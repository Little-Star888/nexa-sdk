// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package service

import (
	"testing"
	"time"

	geniex_sdk "github.com/qualcomm/GenieX/bindings/go"
	"github.com/qualcomm/GenieX/cli/server/middleware"
	"github.com/qualcomm/GenieX/cli/server/types"
)

// ResolveModelParam receives already-resolved knobs (the handler prefills unset
// request fields from the server defaults), so these tests pass the final
// values directly.

// TestResolveModelParam_PassesLlamaCppValuesThrough verifies that nctx / ngl are
// forwarded verbatim for llama_cpp and the compute alias resolves to a device.
func TestResolveModelParam_PassesLlamaCppValuesThrough(t *testing.T) {
	got, err := ResolveModelParam(geniex_sdk.RuntimeLlamaCpp, "some-model", 2048, 10, "gpu", "", types.SpecParam{})
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
	got, err := ResolveModelParam(geniex_sdk.RuntimeLlamaCpp, "some-model", 4096, -1, "npu", "", types.SpecParam{})
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
	got, err := ResolveModelParam(geniex_sdk.RuntimeLlamaCpp, "some-model", 4096, 0, "cpu", "", types.SpecParam{})
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
	got, err := ResolveModelParam(geniex_sdk.RuntimeQairt, "some-model", 8192, 42, "", "", types.SpecParam{})
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

// Regression test for #1322: model destruction shares the request GIL, so
// the cleanup goroutine can never destroy a model a handler is still using.

type fakeModel struct{ destroyed int }

func (f *fakeModel) Destroy() error { f.destroyed++; return nil }

func TestSweepNeverDestroysMidRequest(t *testing.T) {
	f := &fakeModel{}
	keepAlive.name = "m"
	keepAlive.model = &modelKeepInfo{model: f, lastTime: time.Now().Add(-time.Hour)}
	keepAlive.sawBusy = false

	middleware.GILock.Lock() // a request is in flight
	keepAlive.sweep()
	middleware.GILock.Unlock()
	if f.destroyed != 0 {
		t.Fatal("sweep destroyed the model while a request was in flight")
	}

	// The first pass after a busy one only restarts the idle countdown...
	keepAlive.sweep()
	if f.destroyed != 0 {
		t.Fatal("sweep destroyed a model that was in use moments ago")
	}

	// ...so the model survives the next pass as well...
	keepAlive.sweep()
	if f.destroyed != 0 {
		t.Fatal("sweep ignored the restarted idle countdown")
	}

	// ...and is destroyed once genuinely idle past the timeout.
	keepAlive.model.lastTime = time.Now().Add(-time.Hour)
	keepAlive.sweep()
	if f.destroyed != 1 {
		t.Fatal("sweep kept an idle model past the timeout")
	}
	if keepAlive.model != nil {
		t.Fatal("destroyed model still cached")
	}
}
