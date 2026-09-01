// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package config

import "testing"

func TestChipsetDefaults(t *testing.T) {
	tests := []struct {
		name        string
		compute     string
		ubatch      int32
		chipset     string
		wantCompute string
		wantUbatch  int32
	}{
		// compute
		{"explicit npu is kept on rb3", "npu", 0, "qualcomm-qcs6490",
			"npu", 0},
		{"explicit cpu is kept", "cpu", 0, "qualcomm-snapdragon-x-elite",
			"cpu", 0},
		{"whitespace-only compute is treated as unset", "  ", 0, "qualcomm-qcs6490",
			"cpu", 0},
		{"unset on rb3 canonical becomes cpu", "", 0, "qualcomm-qcs6490",
			"cpu", 0},
		{"unset on rb3 display name becomes cpu", "", 0, "Dragonwing RB3 Gen 2 Vision Kit",
			"cpu", 0},
		{"unset on rb3 is case-insensitive", "", 0, "Qualcomm-QCS6490",
			"cpu", 0},
		{"unset on other chipset stays unset", "", 0, "qualcomm-snapdragon-x-elite",
			"", 0},
		{"unset with no detected chipset stays unset", "", 0, "",
			"", 0},

		// ubatch: only an explicit gpu reaches the cap, since an unset compute
		// becomes cpu on these chipsets
		{"unset ubatch on rb3 gpu is capped", "gpu", 0, "qualcomm-qcs6490",
			"gpu", 256},
		{"unset ubatch on rb3 display name is capped", "gpu", 0, "Dragonwing RB3 Gen 2 Vision Kit",
			"gpu", 256},
		{"gpu alias is case-insensitive", "GPU", 0, "qualcomm-qcs6490",
			"GPU", 256},
		{"explicit ubatch is kept on rb3 gpu", "gpu", 512, "qualcomm-qcs6490",
			"gpu", 512},
		{"rb3 cpu keeps the plugin ubatch default", "cpu", 0, "qualcomm-qcs6490",
			"cpu", 0},
		{"rb3 npu keeps the plugin ubatch default", "npu", 0, "qualcomm-qcs6490",
			"npu", 0},
		{"rb3 hybrid keeps the plugin ubatch default", "hybrid", 0, "qualcomm-qcs6490",
			"hybrid", 0},
		{"unset compute defaults to cpu and skips the gpu cap", "", 0, "qualcomm-qcs6490",
			"cpu", 0},
		{"gpu on other chipset is left alone", "gpu", 0, "qualcomm-snapdragon-x-elite",
			"gpu", 0},
		{"gpu with no detected chipset is left alone", "gpu", 0, "",
			"gpu", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			compute, ubatch := ChipsetDefaults(tt.compute, tt.ubatch, tt.chipset)
			if compute != tt.wantCompute || ubatch != tt.wantUbatch {
				t.Fatalf("ChipsetDefaults(%q, %d, %q) = (%q, %d), want (%q, %d)",
					tt.compute, tt.ubatch, tt.chipset, compute, ubatch, tt.wantCompute, tt.wantUbatch)
			}
		})
	}
}
