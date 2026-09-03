// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package main

import (
	"testing"

	geniex_sdk "github.com/qualcomm/GenieX/bindings/go"
)

func TestModelLoadedLine(t *testing.T) {
	tests := []struct {
		name                   string
		runtimeID, computeUnit string
		ngl, nctx              int32
		want                   string
	}{
		{
			name:      "llama_cpp npu shows alias, ngl/nctx",
			runtimeID: geniex_sdk.RuntimeLlamaCpp, computeUnit: "npu", ngl: 32, nctx: 4096,
			want: "Model loaded: runtime=llama_cpp compute=npu ngl=32 nctx=4096",
		},
		{
			name:      "llama_cpp cpu echoes alias (device_id is empty)",
			runtimeID: geniex_sdk.RuntimeLlamaCpp, computeUnit: "cpu", ngl: 0, nctx: 2048,
			want: "Model loaded: runtime=llama_cpp compute=cpu ngl=0 nctx=2048",
		},
		{
			name:      "llama_cpp hybrid echoes alias (device_id is empty)",
			runtimeID: geniex_sdk.RuntimeLlamaCpp, computeUnit: "hybrid", ngl: -1, nctx: 4096,
			want: "Model loaded: runtime=llama_cpp compute=hybrid ngl=-1 nctx=4096",
		},
		{
			name:      "llama_cpp empty compute defaults to npu",
			runtimeID: geniex_sdk.RuntimeLlamaCpp, computeUnit: "", ngl: -1, nctx: 4096,
			want: "Model loaded: runtime=llama_cpp compute=npu ngl=-1 nctx=4096",
		},
		{
			name:      "qairt fixed to npu, hides ngl/nctx",
			runtimeID: geniex_sdk.RuntimeQairt, computeUnit: "cpu", ngl: 0, nctx: 0,
			want: "Model loaded: runtime=qairt compute=npu",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := modelLoadedLine(tt.runtimeID, tt.computeUnit, tt.ngl, tt.nctx)
			if got != tt.want {
				t.Errorf("modelLoadedLine\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

// Precedence for the QAIRT runtime directory. The `qnn-lib` config default exists so
// a machine pinned to one QAIRT version need not repeat --qnn-lib on every run, which
// is exactly why the per-run sources have to outrank it: otherwise a path stored months
// ago would silently beat what the user just typed.
func TestResolveQnnLib(t *testing.T) {
	const flagPath, envPath, configPath = "D:/qairt/2.49", "D:/qairt/2.48", "D:/qairt/2.45"

	tests := []struct {
		name                             string
		flagValue, envValue, configValue string
		want                             string
	}{
		{"nothing set stays on the bundled runtime", "", "", "", ""},
		{"flag alone", flagPath, "", "", flagPath},
		{"env alone", "", envPath, "", envPath},
		{"config default alone", "", "", configPath, configPath},
		{"flag beats env", flagPath, envPath, "", flagPath},
		{"flag beats the config default", flagPath, "", configPath, flagPath},
		{"env beats the config default", "", envPath, configPath, envPath},
		{"flag beats both", flagPath, envPath, configPath, flagPath},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveQnnLib(tt.flagValue, tt.envValue, tt.configValue)
			if got != tt.want {
				t.Errorf("resolveQnnLib(%q, %q, %q)\n got: %q\nwant: %q",
					tt.flagValue, tt.envValue, tt.configValue, got, tt.want)
			}
		})
	}
}
