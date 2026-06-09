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

package handler

import (
	"testing"

	"github.com/openai/openai-go/v3"

	geniex_sdk "github.com/qcom-it-nexa-ai/geniex/bindings/go"
)

func TestParseToolCalls(t *testing.T) {
	cases := []struct {
		name      string
		resp      string
		wantName  string
		wantArgs  string
		wantError bool
	}{
		{
			name:     "tool_call tag with object arguments",
			resp:     `<tool_call>{"name":"get_weather","arguments":{"city":"SF"}}</tool_call>`,
			wantName: "get_weather",
			wantArgs: `{"city":"SF"}`,
		},
		{
			name:     "tool_call tag with string arguments",
			resp:     `<tool_call>{"name":"echo","arguments":"hello"}</tool_call>`,
			wantName: "echo",
			wantArgs: "hello",
		},
		{
			name:     "fenced json block",
			resp:     "```json\n{\"name\":\"ping\",\"arguments\":{}}\n```",
			wantName: "ping",
			wantArgs: "{}",
		},
		{
			name:     "tool_call wins when both present",
			resp:     "<tool_call>{\"name\":\"a\",\"arguments\":{}}</tool_call> ```json{\"name\":\"b\",\"arguments\":{}}```",
			wantName: "a",
			wantArgs: "{}",
		},
		{
			name:      "no match",
			resp:      "just some plain text",
			wantError: true,
		},
		{
			name:      "missing name field",
			resp:      `<tool_call>{"arguments":{"x":1}}</tool_call>`,
			wantError: true,
		},
		{
			name:      "missing arguments field",
			resp:      `<tool_call>{"name":"foo"}</tool_call>`,
			wantError: true,
		},
		{
			name:      "arguments wrong type (number)",
			resp:      `<tool_call>{"name":"foo","arguments":5}</tool_call>`,
			wantError: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseToolCalls(tc.resp)
			if tc.wantError {
				if err == nil {
					t.Fatalf("expected error, got %+v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Name != tc.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tc.wantName)
			}
			if got.Arguments != tc.wantArgs {
				t.Errorf("Arguments = %q, want %q", got.Arguments, tc.wantArgs)
			}
		})
	}
}

func TestMapFinishReason(t *testing.T) {
	cases := map[string]string{
		"length":        "length",
		"user":          "stop",
		"eos":           "stop",
		"stop_sequence": "stop",
		"":              "stop",
		"something_new": "stop",
	}
	for in, want := range cases {
		if got := mapFinishReason(in); got != want {
			t.Errorf("mapFinishReason(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestProfile2Usage(t *testing.T) {
	p := geniex_sdk.ProfileData{PromptTokens: 12, GeneratedTokens: 30}
	u := profile2Usage(p)
	if u.PromptTokens != 12 {
		t.Errorf("PromptTokens = %d, want 12", u.PromptTokens)
	}
	if u.CompletionTokens != 30 {
		t.Errorf("CompletionTokens = %d, want 30", u.CompletionTokens)
	}
	if u.TotalTokens != 42 {
		t.Errorf("TotalTokens = %d, want 42", u.TotalTokens)
	}
}

func TestParseSamplerConfig(t *testing.T) {
	req := defaultChatCompletionRequest()
	req.TopK = 40
	req.MinP = 0.05
	req.RepetitionPenalty = 1.1
	req.EnableJson = true
	req.Temperature = openai.Float(0.7)
	req.TopP = openai.Float(0.9)
	req.Seed = openai.Int(123)

	sc := parseSamplerConfig(req)
	if sc.TopK != 40 {
		t.Errorf("TopK = %d, want 40", sc.TopK)
	}
	if sc.MinP != 0.05 {
		t.Errorf("MinP = %v, want 0.05", sc.MinP)
	}
	if sc.RepetitionPenalty != 1.1 {
		t.Errorf("RepetitionPenalty = %v, want 1.1", sc.RepetitionPenalty)
	}
	if !sc.EnableJson {
		t.Error("EnableJson = false, want true")
	}
	if sc.Temperature != 0.7 {
		t.Errorf("Temperature = %v, want 0.7", sc.Temperature)
	}
	if sc.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", sc.TopP)
	}
	if sc.Seed != 123 {
		t.Errorf("Seed = %d, want 123", sc.Seed)
	}
}

func TestParseTools(t *testing.T) {
	t.Run("no tools", func(t *testing.T) {
		req := defaultChatCompletionRequest()
		enabled, tools, err := parseTools(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if enabled {
			t.Error("enabled = true, want false for empty tools")
		}
		if tools != "" {
			t.Errorf("tools = %q, want empty", tools)
		}
	})

	t.Run("with a tool", func(t *testing.T) {
		req := defaultChatCompletionRequest()
		req.Tools = []openai.ChatCompletionToolUnionParam{
			openai.ChatCompletionFunctionTool(openai.FunctionDefinitionParam{
				Name: "get_weather",
			}),
		}
		enabled, tools, err := parseTools(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !enabled {
			t.Error("enabled = false, want true")
		}
		if tools == "" {
			t.Error("tools serialized to empty string")
		}
	})
}

func TestIsWarmupRequest(t *testing.T) {
	t.Run("no messages is warmup", func(t *testing.T) {
		if !isWarmupRequest(defaultChatCompletionRequest()) {
			t.Error("empty messages should be a warmup request")
		}
	})

	t.Run("single system message is warmup", func(t *testing.T) {
		req := defaultChatCompletionRequest()
		req.Messages = []openai.ChatCompletionMessageParamUnion{systemMessage("you are a helpful assistant")}
		if !isWarmupRequest(req) {
			t.Error("single system message should be a warmup request")
		}
	})

	t.Run("single user message is not warmup", func(t *testing.T) {
		req := defaultChatCompletionRequest()
		req.Messages = []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hello"),
		}
		if isWarmupRequest(req) {
			t.Error("single user message should not be a warmup request")
		}
	})

	t.Run("multiple messages are not warmup", func(t *testing.T) {
		req := defaultChatCompletionRequest()
		req.Messages = []openai.ChatCompletionMessageParamUnion{
			systemMessage("sys"),
			openai.UserMessage("hi"),
		}
		if isWarmupRequest(req) {
			t.Error("multiple messages should not be a warmup request")
		}
	})
}

// systemMessage builds a system message with the Role field explicitly set.
// openai.SystemMessage leaves Role as the zero value (it only materializes to
// "system" on JSON marshal), but isWarmupRequest reads Role via GetRole, which
// mirrors what the handler sees after binding a real request body.
func systemMessage(content string) openai.ChatCompletionMessageParamUnion {
	msg := openai.SystemMessage(content)
	msg.OfSystem.Role = "system"
	return msg
}

func TestDefaultChatCompletionRequest(t *testing.T) {
	req := defaultChatCompletionRequest()
	if req.MaxCompletionTokens.Value != 2048 {
		t.Errorf("MaxCompletionTokens = %d, want 2048", req.MaxCompletionTokens.Value)
	}
	if !req.EnableThink {
		t.Error("EnableThink = false, want true")
	}
	if req.ImageMaxLength != 512 {
		t.Errorf("ImageMaxLength = %d, want 512", req.ImageMaxLength)
	}
	if req.RepetitionPenalty != 1.0 {
		t.Errorf("RepetitionPenalty = %v, want 1.0", req.RepetitionPenalty)
	}
	if req.Stream {
		t.Error("Stream = true, want false")
	}
}
