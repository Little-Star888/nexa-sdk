// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package utils

// Qwen3ToolCall is the `<tool_call>{json}</tool_call>` wrapper of Qwen3 and
// Hermes-style templates. Its body is plain JSON, so it shares that parser.
type Qwen3ToolCall struct {
	markerFormat
}

func newQwen3ToolCall() *Qwen3ToolCall {
	return &Qwen3ToolCall{newMarkerFormat("<tool_call>", "</tool_call>")}
}

func (t *Qwen3ToolCall) parse(s string) []toolCallFn { return parseJSONToolCalls(s) }
