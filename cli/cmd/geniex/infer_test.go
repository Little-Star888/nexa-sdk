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

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// loadStopSequences reads --stop flags and the --stop-file, combining both.
// These are package-level vars, so each subtest resets them.
func TestLoadStopSequences(t *testing.T) {
	reset := func() { stop = nil; stopFile = "" }

	t.Run("no sources", func(t *testing.T) {
		reset()
		got, err := loadStopSequences()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %v, want empty", got)
		}
	})

	t.Run("only --stop flags", func(t *testing.T) {
		reset()
		stop = []string{"</s>", "STOP"}
		got, err := loadStopSequences()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"</s>", "STOP"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("stop-file with blank lines skipped", func(t *testing.T) {
		reset()
		dir := t.TempDir()
		path := filepath.Join(dir, "stops.txt")
		if err := os.WriteFile(path, []byte("END\n\n<eos>\n"), 0644); err != nil {
			t.Fatal(err)
		}
		stopFile = path
		got, err := loadStopSequences()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"END", "<eos>"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("stop-file lines come before --stop flags", func(t *testing.T) {
		reset()
		dir := t.TempDir()
		path := filepath.Join(dir, "stops.txt")
		if err := os.WriteFile(path, []byte("FILE1\nFILE2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		stopFile = path
		stop = []string{"FLAG1"}
		got, err := loadStopSequences()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"FILE1", "FILE2", "FLAG1"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("missing stop-file errors", func(t *testing.T) {
		reset()
		stopFile = filepath.Join(t.TempDir(), "does-not-exist.txt")
		if _, err := loadStopSequences(); err == nil {
			t.Error("expected error for missing stop-file, got nil")
		}
	})
}
