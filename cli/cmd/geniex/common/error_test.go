// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package common

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	geniex_sdk "github.com/qcom-it-nexa-ai/geniex/bindings/go"
	"github.com/qcom-it-nexa-ai/geniex/cli/internal/model_hub"
	"github.com/qcom-it-nexa-ai/geniex/cli/internal/testutil"
)

func TestPrintError_Nil(t *testing.T) {
	_, stderr, _ := testutil.CaptureOutput(t, func() error {
		PrintError(nil)
		return nil
	})
	if stderr != "" {
		t.Errorf("PrintError(nil) wrote %q, want nothing", stderr)
	}
}

// A known sentinel must render its friendly hint rather than the raw error.
func TestPrintError_SentinelHints(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		snippet string
	}{
		{"param not supported", geniex_sdk.ErrCommonParamNotSupported, "not supported by the plugin"},
		{"not support", geniex_sdk.ErrCommonNotSupport, "not supported yet"},
		{"model load", geniex_sdk.ErrCommonModelLoad, "Model failed to load"},
		{"plugin load", geniex_sdk.ErrCommonPluginLoad, "Plugin failed to load"},
		{"plugin invalid", geniex_sdk.ErrCommonPluginInvalid, "Plugin is invalid"},
		{"context length", geniex_sdk.ErrLlmTokenizationContextLength, "Context length exceeded"},
		{"hub unreachable", model_hub.ErrUnreachable, "Unable to reach the model hub"},
		{"hub auth required", model_hub.ErrAuthRequired, "auth required"},
		{"model not found", model_hub.ErrModelNotFound, "Model not found on the hub"},
		{"server unreachable", ErrServerUnreachable, "Could not reach the geniex server"},
		{"precision not found", ErrPrecisionNotFound, "precision is not available locally"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, _ := testutil.CaptureOutput(t, func() error {
				PrintError(tc.err)
				return nil
			})
			if !strings.Contains(stderr, tc.snippet) {
				t.Errorf("hint for %v = %q, want it to contain %q", tc.err, stderr, tc.snippet)
			}
		})
	}
}

// Sentinels are matched through wrapping (errors.Is via %w).
func TestPrintError_WrappedSentinel(t *testing.T) {
	wrapped := fmt.Errorf("download step: %w", model_hub.ErrModelNotFound)
	_, stderr, _ := testutil.CaptureOutput(t, func() error {
		PrintError(wrapped)
		return nil
	})
	if !strings.Contains(stderr, "Model not found on the hub") {
		t.Errorf("wrapped sentinel not matched, got %q", stderr)
	}
}

// An unknown error falls back to the raw "Error: <msg>" form.
func TestPrintError_UnknownFallsBackToRaw(t *testing.T) {
	_, stderr, _ := testutil.CaptureOutput(t, func() error {
		PrintError(errors.New("some unexpected failure"))
		return nil
	})
	if !strings.Contains(stderr, "Error: some unexpected failure") {
		t.Errorf("fallback render = %q, want raw error message", stderr)
	}
}

// The first matching sentinel wins; the raw message must not also appear.
func TestPrintError_SentinelSuppressesRaw(t *testing.T) {
	_, stderr, _ := testutil.CaptureOutput(t, func() error {
		PrintError(ErrServerUnreachable)
		return nil
	})
	if strings.Contains(stderr, "Error: server unreachable") {
		t.Errorf("raw message leaked alongside hint: %q", stderr)
	}
}
