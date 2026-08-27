// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package config

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/qualcomm/GenieX/cli/internal/render"
)

// chipsetOverrides replaces flag defaults that don't fit a chipset. Keys are
// lower-cased and cover both forms ResolveChipset yields: the canonical id and
// the reference-device name.
var chipsetOverrides = map[string]struct {
	compute   string // what an unset --compute becomes, where the NPU path is slow
	gpuUbatch int32  // what an unset n_ubatch becomes on GPU, where the default overruns max alloc
}{
	// Adreno 643 caps one allocation at 256 MiB.
	"qualcomm-qcs6490":                {compute: "cpu", gpuUbatch: 256},
	"dragonwing rb3 gen 2 vision kit": {compute: "cpu", gpuUbatch: 256},
}

// ChipsetDefaults fills unset flags from the chipset's entry, reporting each
// substitution. Ubatch resolves against the returned compute, so a chipset
// defaulting to cpu never takes the GPU-only cap.
func ChipsetDefaults(compute string, ubatch int32, chipset string) (string, int32) {
	over, ok := chipsetOverrides[strings.ToLower(strings.TrimSpace(chipset))]
	if !ok {
		return compute, ubatch
	}

	if strings.TrimSpace(compute) == "" && over.compute != "" {
		compute = over.compute
		slog.Info("applied host-aware compute default", "compute", compute, "chipset", chipset)
		fmt.Println(render.GetTheme().Info.Sprintf(
			"Defaulting to --compute %s for this device; pass --compute to override.", compute))
	}
	if ubatch <= 0 && over.gpuUbatch > 0 && strings.EqualFold(strings.TrimSpace(compute), "gpu") {
		ubatch = over.gpuUbatch
		slog.Info("applied host-aware ubatch cap", "n_ubatch", ubatch, "chipset", chipset)
		fmt.Println(render.GetTheme().Info.Sprintf(
			"Capping n_ubatch to %d for this device's GPU.", ubatch))
	}
	return compute, ubatch
}
