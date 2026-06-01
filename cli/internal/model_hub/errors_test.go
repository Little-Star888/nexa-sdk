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

package model_hub

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/qcom-it-nexa-ai/geniex/cli/internal/model_hub/aihub"
)

func TestTranslateAIHubError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		// want is the hub sentinel the result must match via errors.Is.
		// nil means the error must pass through unchanged.
		want error
	}{
		{"aihub not-found sentinel", fmt.Errorf("lookup: %w", aihub.ErrModelNotFound), ErrModelNotFound},
		{"http 404", &aihub.HTTPError{URL: "u", Status: 404}, ErrModelNotFound},
		{"http 401", &aihub.HTTPError{URL: "u", Status: 401}, ErrAuthRequired},
		{"http 403", &aihub.HTTPError{URL: "u", Status: 403}, ErrAuthRequired},
		{"http 500", &aihub.HTTPError{URL: "u", Status: 500}, ErrUnreachable},
		{"http 400", &aihub.HTTPError{URL: "u", Status: 400}, ErrUnreachable},
		{"unrelated passes through", errors.New("some other failure"), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := TranslateAIHubError(c.in)
			if c.want == nil {
				if got != c.in {
					t.Errorf("TranslateAIHubError(%v) = %v, want unchanged input", c.in, got)
				}
				for _, s := range []error{ErrModelNotFound, ErrAuthRequired, ErrUnreachable} {
					if errors.Is(got, s) {
						t.Errorf("TranslateAIHubError(%v) unexpectedly matched sentinel %v", c.in, s)
					}
				}
				return
			}
			if !errors.Is(got, c.want) {
				t.Errorf("TranslateAIHubError(%v) = %v, want errors.Is %v", c.in, got, c.want)
			}
		})
	}
}

func TestWrapTransport(t *testing.T) {
	t.Run("nil passes through", func(t *testing.T) {
		if got := wrapTransport("http://h.co", nil); got != nil {
			t.Errorf("wrapTransport(nil) = %v, want nil", got)
		}
	})

	// Errors already tagged with a hub sentinel must pass through untouched so
	// middleware-produced errors keep their original tag.
	for _, sentinel := range []error{ErrUnreachable, ErrAuthRequired, ErrModelNotFound} {
		t.Run("already tagged "+sentinel.Error(), func(t *testing.T) {
			in := fmt.Errorf("%w: detail", sentinel)
			got := wrapTransport("http://h.co", in)
			if got != in {
				t.Errorf("wrapTransport(%v) = %v, want unchanged input", in, got)
			}
			if !errors.Is(got, sentinel) {
				t.Errorf("wrapTransport(%v) lost sentinel %v", in, sentinel)
			}
		})
	}

	t.Run("plain error wrapped as unreachable", func(t *testing.T) {
		in := errors.New("dial tcp: i/o timeout")
		got := wrapTransport("http://h.co/x", in)
		if !errors.Is(got, ErrUnreachable) {
			t.Errorf("wrapTransport(%v) = %v, want errors.Is ErrUnreachable", in, got)
		}
		if got == in {
			t.Error("wrapTransport returned the input unchanged, want wrapped")
		}
		if msg := got.Error(); !strings.Contains(msg, "http://h.co/x") || !strings.Contains(msg, "i/o timeout") {
			t.Errorf("wrapTransport message = %q, want url and original message", msg)
		}
	})
}

func TestNormalizeModelName(t *testing.T) {
	// Relies on config.GetModelMapping returning false for every input
	// (modelMappingSrc is empty in this build), so the mapping branch is not
	// exercised here.
	cases := []struct {
		name      string
		in        string
		wantName  string
		wantQuant string
	}{
		{"bare name", "llama", "qualcomm/llama", ""},
		{"org/repo unchanged", "acme/llama", "acme/llama", ""},
		{"bare name with quant", "llama:q4_0", "qualcomm/llama", "Q4_0"},
		{"org/repo with quant", "acme/llama:q8_0", "acme/llama", "Q8_0"},
		{"hf url", HF_ENDPOINT + "/acme/llama", "acme/llama", ""},
		{"hf url with quant", HF_ENDPOINT + "/acme/llama:q4_k_m", "acme/llama", "Q4_K_M"},
		{"ai-hub-models prefix canonicalized", "ai-hub-models/llama", "qualcomm/llama", ""},
		{"ai-hub-models prefix mixed case", "AI-Hub-Models/llama:q4_0", "qualcomm/llama", "Q4_0"},
		{"qualcomm prefix unchanged", "qualcomm/llama", "qualcomm/llama", ""},
		{"qualcomm prefix mixed case", "Qualcomm/Llama", "qualcomm/Llama", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gotName, gotQuant := NormalizeModelName(c.in)
			if gotName != c.wantName || gotQuant != c.wantQuant {
				t.Errorf("NormalizeModelName(%q) = (%q, %q), want (%q, %q)",
					c.in, gotName, gotQuant, c.wantName, c.wantQuant)
			}
		})
	}
}
