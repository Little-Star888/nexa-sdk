// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package service

import (
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"time"

	geniex_sdk "github.com/qualcomm/GenieX/bindings/go"
	"github.com/qualcomm/GenieX/cli/internal/config"
	"github.com/qualcomm/GenieX/cli/internal/render"
	"github.com/qualcomm/GenieX/cli/internal/types"
	"github.com/qualcomm/GenieX/cli/server/middleware"
)

// ResolveModelParam turns the (nctx, ngl, compute) knobs into the ModelParam
// the keep-alive cache keys on. The caller passes already-resolved values (the
// handler prefills unset request fields with the server-wide --nctx / --ngl /
// --compute defaults). NCtx / NGpuLayers are meaningful only for llama_cpp; for
// other plugins (e.g. qairt) NCtx is zeroed here and the SDK zeroes ngl so the
// plugin's param-guard is not tripped. Compute is resolved to a concrete
// DeviceID by the SDK (sdk/src/device.cpp); any coercion warning is logged and
// printed to stdout.
// SpecParam bundles the speculative-decoding knobs sourced from a request; all
// zero-values mean "spec disabled". Only llama_cpp consumes these fields.
type SpecParam struct {
	Type       string
	DraftModel string
	NMax       int32
	NMin       int32
	PMin       float32
}

// resolveDraftModelPath maps a request's spec_draft_model value to an absolute
// GGUF path. An existing filesystem path is returned as-is; anything else is
// treated as a catalogue name (optionally suffixed with :precision) and looked
// up in the local cache. A missing cache entry returns an error - the caller
// is expected to `geniex pull` the draft model beforehand, matching how the
// target model is handled (server never auto-pulls; it consumes what's cached).
func resolveDraftModelPath(draft string) (string, error) {
	if draft == "" {
		return "", nil
	}
	if _, err := os.Stat(draft); err == nil {
		return draft, nil
	}
	name, precision := geniex_sdk.SplitNamePrecision(draft)
	key := name
	if precision != "" {
		key = name + ":" + precision
	}
	paths, err := geniex_sdk.ModelGetPaths(key)
	if err != nil {
		return "", fmt.Errorf("resolve draft model %q: %w", draft, err)
	}
	return paths.ModelPath, nil
}

func ResolveModelParam(runtimeID, modelName string, reqNCtx, reqNgl int32, reqCompute, chipset string, spec SpecParam) (types.ModelParam, error) {
	// nctx / ngl / compute already carry the resolved value (explicit request
	// or the server default prefilled by the handler). Non-llama_cpp plugins
	// (e.g. qairt) reject non-zero nctx, so zero it for them; the SDK does the
	// same for ngl in geniex_resolve_device.
	nctx, ngl := reqNCtx, reqNgl
	if runtimeID != geniex_sdk.RuntimeLlamaCpp {
		nctx = 0
	}

	// Host-aware default (e.g. RB3 Gen 2 → cpu) before the SDK's npu fallback.
	// chipset is resolved by the caller (offline) so this stays store-free.
	if resolved, overridden := config.ComputeDefault(reqCompute, chipset); overridden {
		slog.Info("applied host-aware compute default", "compute", resolved)
		fmt.Println(render.GetTheme().Info.Sprintf("Defaulting to compute %s for this device.", resolved))
		reqCompute = resolved
	}

	resolved, err := geniex_sdk.ResolveDevice(geniex_sdk.ResolveDeviceInput{
		RuntimeID:   runtimeID,
		ModelName:   modelName,
		ComputeUnit: reqCompute,
		NglDefault:  ngl,
	})
	if err != nil {
		return types.ModelParam{}, err
	}
	if resolved.Warning != "" {
		slog.Warn("compute unit coerced", "warning", resolved.Warning)
		fmt.Println(render.GetTheme().Warning.Sprintf("Warning: %s", resolved.Warning))
	}

	mp := types.ModelParam{
		NCtx:       nctx,
		NGpuLayers: resolved.Ngl,
		DeviceID:   resolved.DeviceID,
	}
	if runtimeID == geniex_sdk.RuntimeLlamaCpp {
		mp.SpecType = spec.Type
		mp.SpecDraftModel = spec.DraftModel
		mp.SpecNMax = spec.NMax
		mp.SpecNMin = spec.NMin
		mp.SpecPMin = spec.PMin
	}
	return mp, nil
}

// KeepAliveGet retrieves a model from the keepalive cache or creates it if not found
// This avoids the overhead of repeatedly loading/unloading models from disk
func KeepAliveGet[T any](name string, param types.ModelParam, reset bool) (*T, error) {
	t, err := keepAliveGet[T](name, param, reset)
	if err != nil {
		return nil, err
	}
	return t.(*T), nil
}

var keepAlive keepAliveService

// keepAliveService caches a single model: geniex serve supports one model
// architecture by design, so the cache is one slot and the request GIL
// (middleware.GILock) is the only lock guarding it. Handlers hold the GIL
// for the whole request via middleware.GIL; the cleanup goroutine takes it
// before destroying, so a model is never destroyed mid-generation (#1322).
type keepAliveService struct {
	name    string         // name key of the cached model
	model   *modelKeepInfo // the cached model; nil when none
	sawBusy bool           // owned by the cleanup goroutine: last pass found a request in flight
	stopCh  chan struct{}  // Channel to stop the cleanup goroutine
}

// modelKeepInfo holds metadata for a cached model instance
type modelKeepInfo struct {
	model    keepable
	param    types.ModelParam
	lastTime time.Time
}

// keepable interface defines objects that can be managed by the keepalive service
// Objects must support cleanup and reset operations
type keepable interface {
	Destroy() error
}

type keepResetable interface {
	keepable
	Reset() error
}

// start begins the background cleanup process that removes the unused model
// Runs a ticker every 5 seconds to check whether it exceeds the keepalive timeout
func (keepAlive *keepAliveService) start() {
	keepAlive.stopCh = make(chan struct{})

	go func() {
		t := time.NewTicker(5 * time.Second)
		for {
			select {
			// Stop signal received - cleanup the model and exit
			case <-keepAlive.stopCh:
				middleware.GILock.Lock()
				if keepAlive.model != nil {
					keepAlive.model.model.Destroy()
					keepAlive.model = nil
				}
				middleware.GILock.Unlock()
				return

			// Periodic cleanup - remove the model if it hasn't been used recently
			case <-t.C:
				keepAlive.sweep()
			}
		}
	}()
}

// sweep destroys the cached model once it is idle past the keepalive
// timeout. It shares the request GIL with the handlers (middleware.GIL):
// while a request is in flight the pass is skipped, and the first pass
// after a busy one only restarts the idle countdown. A model is therefore
// never destroyed mid-generation, and a generation that outlives the
// timeout does not consume its own keep-alive window (#1322).
func (keepAlive *keepAliveService) sweep() {
	if !middleware.GILock.TryLock() {
		keepAlive.sawBusy = true
		return
	}
	defer middleware.GILock.Unlock()

	if keepAlive.model == nil {
		keepAlive.sawBusy = false
		return
	}
	if keepAlive.sawBusy {
		keepAlive.sawBusy = false
		keepAlive.model.lastTime = time.Now()
		return
	}
	if time.Since(keepAlive.model.lastTime).Milliseconds()/1000 > config.Get().KeepAlive {
		keepAlive.model.model.Destroy()
		keepAlive.model = nil
	}
}

// keepAliveGet retrieves the cached model or creates a new one if not found.
// Callers must hold the request GIL; middleware.GIL does this for every
// handler, so the single cache slot needs no lock of its own.
func keepAliveGet[T any](name string, param types.ModelParam, reset bool) (any, error) {
	// The SDK resolves bare names / aliases and picks the default precision
	// when none is given; pass the request string through verbatim.
	paths, err := geniex_sdk.ModelGetPaths(name)
	if err != nil {
		return nil, err
	}
	slog.Debug("KeepAliveGet", "name", name, "param", param, "model_path", paths.ModelPath)

	modelfile := paths.ModelPath

	// Reuse the cached model when the request matches it
	if keepAlive.model != nil && keepAlive.name == name && reflect.DeepEqual(keepAlive.model.param, param) {
		if reset {
			if r, ok := keepAlive.model.model.(keepResetable); ok {
				r.Reset()
			}
		}
		keepAlive.model.lastTime = time.Now()
		return keepAlive.model.model, nil
	}

	// A different model or params were requested: destroy the current model
	// first so only one is ever in memory
	// TODO: unload model due to free ram/vram
	if keepAlive.model != nil {
		keepAlive.model.model.Destroy()
		keepAlive.model = nil
	}

	// param already carries the resolved NCtx / NGpuLayers / DeviceID (see
	// resolveServeModelParam in the chat handler); the cache keys on it, so no
	// further resolution happens here.
	var t keepable
	var e error
	switch reflect.TypeFor[T]() {
	case reflect.TypeFor[geniex_sdk.LLM]():
		draftPath := ""
		if param.SpecType != "" && param.SpecDraftModel != "" {
			p, perr := resolveDraftModelPath(param.SpecDraftModel)
			if perr != nil {
				return nil, perr
			}
			draftPath = p
		}
		t, e = geniex_sdk.NewLLM(geniex_sdk.LlmCreateInput{
			ModelPath: modelfile,
			DeviceID:  param.DeviceID,
			Config: geniex_sdk.ModelConfig{
				NCtx:           param.NCtx,
				NGpuLayers:     param.NGpuLayers,
				SpecType:       param.SpecType,
				SpecDraftModel: draftPath,
				SpecNMax:       param.SpecNMax,
				SpecNMin:       param.SpecNMin,
				SpecPMin:       param.SpecPMin,
			},
			RuntimeID: paths.RuntimeID,
		})
	case reflect.TypeFor[geniex_sdk.VLM]():
		t, e = geniex_sdk.NewVLM(geniex_sdk.VlmCreateInput{
			ModelPath:  modelfile,
			MmprojPath: paths.MmprojPath,
			DeviceID:   param.DeviceID,
			Config: geniex_sdk.ModelConfig{
				NCtx:       param.NCtx,
				NGpuLayers: param.NGpuLayers,
			},
			RuntimeID: paths.RuntimeID,
		})
	default:
		return nil, fmt.Errorf("unsupported model type: %s", reflect.TypeFor[T]())
	}
	if e != nil {
		return nil, e
	}
	keepAlive.name = name
	keepAlive.model = &modelKeepInfo{
		model:    t,
		param:    param,
		lastTime: time.Now(),
	}

	return keepAlive.model.model, nil
}

// stop signals the cleanup goroutine to terminate
func (keepAlive *keepAliveService) stop() {
	keepAlive.stopCh <- struct{}{}
}
