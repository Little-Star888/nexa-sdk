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

package render

import (
	"bytes"
	"fmt"
	"testing"
)

func TestNoColorMethods(t *testing.T) {
	var n NoColor

	t.Run("Sprint", func(t *testing.T) {
		cases := [][]any{
			{"hi"},
			{1, 2},
			{"a", 1},
		}
		for _, args := range cases {
			// Compare against fmt.Sprint to sidestep its spacing rules
			// (spaces only between non-string operands).
			if got, want := n.Sprint(args...), fmt.Sprint(args...); got != want {
				t.Errorf("NoColor.Sprint(%v) = %q, want %q", args, got, want)
			}
		}
	})

	t.Run("Sprintf", func(t *testing.T) {
		if got := n.Sprintf("%s-%d", "x", 3); got != "x-3" {
			t.Errorf("NoColor.Sprintf = %q, want %q", got, "x-3")
		}
	})

	t.Run("Code", func(t *testing.T) {
		if got := n.Code(); got != "" {
			t.Errorf("NoColor.Code() = %q, want empty", got)
		}
	})
}

func TestNewStyledWriterPassthrough(t *testing.T) {
	t.Run("single write", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewStyledWriter(&buf, NoColor{})
		n, err := w.Write([]byte("hello"))
		if err != nil {
			t.Fatalf("Write: %v", err)
		}
		// Contract: Write reports len(p), regardless of bytes the style adds.
		if n != 5 {
			t.Errorf("n = %d, want 5", n)
		}
		if buf.String() != "hello" {
			t.Errorf("buf = %q, want %q", buf.String(), "hello")
		}
	})

	t.Run("accumulates across writes", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewStyledWriter(&buf, NoColor{})
		for _, chunk := range []string{"foo", "bar"} {
			n, err := w.Write([]byte(chunk))
			if err != nil {
				t.Fatalf("Write(%q): %v", chunk, err)
			}
			if n != len(chunk) {
				t.Errorf("Write(%q) n = %d, want %d", chunk, n, len(chunk))
			}
		}
		if buf.String() != "foobar" {
			t.Errorf("buf = %q, want %q", buf.String(), "foobar")
		}
	})

	t.Run("empty write", func(t *testing.T) {
		var buf bytes.Buffer
		w := NewStyledWriter(&buf, NoColor{})
		n, err := w.Write(nil)
		if err != nil {
			t.Fatalf("Write(nil): %v", err)
		}
		if n != 0 {
			t.Errorf("n = %d, want 0", n)
		}
		if buf.Len() != 0 {
			t.Errorf("buf = %q, want empty", buf.String())
		}
	})
}
