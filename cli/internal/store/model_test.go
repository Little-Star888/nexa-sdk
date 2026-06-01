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

package store

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/qcom-it-nexa-ai/geniex/cli/internal/types"
)

// newStore returns a Store rooted at a temp dir, bypassing the Get() singleton
// (whose init() reads config.Get() and os.Exit(1)s on failure).
func newStore(t *testing.T) *Store {
	t.Helper()
	return &Store{home: t.TempDir()}
}

// seedManifest creates the model directory and writes mf as its manifest.
// writeManifest does not mkdir, so the directory is created first.
func seedManifest(t *testing.T, s *Store, name string, mf types.ModelManifest) {
	t.Helper()
	if err := os.MkdirAll(s.ModelfilePath(name, ""), 0o770); err != nil {
		t.Fatalf("mkdir model dir: %v", err)
	}
	if err := s.writeManifest(name, mf); err != nil {
		t.Fatalf("writeManifest: %v", err)
	}
}

// writeModelFile drops a file into the model directory (the on-disk payload a
// manifest entry refers to).
func writeModelFile(t *testing.T, s *Store, name, fname string) {
	t.Helper()
	if err := os.WriteFile(s.ModelfilePath(name, fname), []byte("data"), 0o644); err != nil {
		t.Fatalf("write model file %s: %v", fname, err)
	}
}

func locked(s *Store, name string) bool {
	_, ok := s.modelLocks.Load(name)
	return ok
}

func TestModelPathBuilders(t *testing.T) {
	s := &Store{home: "/data"}
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"ModelfilePath with file", s.ModelfilePath("acme/llama", types.ManifestFileName), filepath.Join("/data", "models", "acme/llama", types.ManifestFileName)},
		{"ModelfilePath empty file", s.ModelfilePath("acme/llama", ""), filepath.Join("/data", "models", "acme/llama")},
		{"ModelDirPath", s.ModelDirPath(), filepath.Join("/data", "models")},
		{"DataPath", s.DataPath(), "/data"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestWriteReadManifestRoundTrip(t *testing.T) {
	s := newStore(t)
	mf := types.ModelManifest{
		Name:      "acme/llama",
		ModelName: "llama-4b",
		ModelType: types.ModelTypeVLM,
		PluginId:  "llama_cpp",
		ModelFile: map[string]types.ModelFileInfo{
			"Q4_0": {Name: "llama-q4_0.gguf", Downloaded: true, Size: 1024},
		},
		MMProjFile: types.ModelFileInfo{Name: "mmproj.gguf", Downloaded: true, Size: 256},
		ExtraFiles: []types.ModelFileInfo{{Name: "tokenizer.json", Downloaded: true, Size: 8}},
	}
	seedManifest(t, s, "acme/llama", mf)

	got, err := s.readManifest("acme/llama")
	if err != nil {
		t.Fatalf("readManifest: %v", err)
	}
	if !reflect.DeepEqual(*got, mf) {
		t.Errorf("round-trip mismatch:\n got %+v\nwant %+v", *got, mf)
	}

	if _, err := s.readManifest("nope/nope"); err == nil {
		t.Error("readManifest of missing model: want error, got nil")
	}
}

func TestGetManifestLocksThenReads(t *testing.T) {
	s := newStore(t)
	mf := types.ModelManifest{Name: "acme/llama", PluginId: "llama_cpp"}
	seedManifest(t, s, "acme/llama", mf)

	got, err := s.GetManifest("acme/llama")
	if err != nil {
		t.Fatalf("GetManifest: %v", err)
	}
	if got.Name != "acme/llama" {
		t.Errorf("Name = %q, want acme/llama", got.Name)
	}
	if locked(s, "acme/llama") {
		t.Error("lock not released after GetManifest")
	}

	// Manifest missing but directory present: still releases the lock.
	if err := os.MkdirAll(s.ModelfilePath("acme/empty", ""), 0o770); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := s.GetManifest("acme/empty"); err == nil {
		t.Error("GetManifest of dir without manifest: want error")
	}
	if locked(s, "acme/empty") {
		t.Error("lock not released after failed GetManifest")
	}
}

func TestList(t *testing.T) {
	t.Run("lists seeded, skips .cache stray and broken", func(t *testing.T) {
		s := newStore(t)
		seedManifest(t, s, "acme/llama", types.ModelManifest{Name: "acme/llama"})
		seedManifest(t, s, "beta/yolo", types.ModelManifest{Name: "beta/yolo"})
		// Broken: directory exists but no manifest -> warned and skipped.
		if err := os.MkdirAll(s.ModelfilePath("acme/broken", ""), 0o770); err != nil {
			t.Fatalf("mkdir broken: %v", err)
		}
		// .cache org dir is ignored; a stray file under models/ is not a dir.
		if err := os.MkdirAll(filepath.Join(s.ModelDirPath(), ".cache"), 0o770); err != nil {
			t.Fatalf("mkdir .cache: %v", err)
		}
		if err := os.WriteFile(filepath.Join(s.ModelDirPath(), "stray.txt"), []byte("x"), 0o644); err != nil {
			t.Fatalf("write stray: %v", err)
		}

		got, err := s.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		names := make([]string, len(got))
		for i, m := range got {
			names[i] = m.Name
		}
		sort.Strings(names)
		want := []string{"acme/llama", "beta/yolo"}
		if !reflect.DeepEqual(names, want) {
			t.Errorf("List names = %v, want %v", names, want)
		}
	})

	t.Run("empty models dir", func(t *testing.T) {
		s := newStore(t)
		if err := os.MkdirAll(s.ModelDirPath(), 0o770); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		got, err := s.List()
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("List = %v, want empty", got)
		}
	})

	t.Run("missing models dir errors", func(t *testing.T) {
		s := newStore(t) // home exists, home/models does not
		if _, err := s.List(); err == nil {
			t.Error("List with no models dir: want error, got nil")
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("not found", func(t *testing.T) {
		s := newStore(t)
		err := s.Remove("ghost/model", "")
		if err == nil || !strings.Contains(err.Error(), "not found") {
			t.Errorf("Remove missing = %v, want 'not found'", err)
		}
	})

	t.Run("whole model", func(t *testing.T) {
		s := newStore(t)
		seedManifest(t, s, "acme/llama", types.ModelManifest{
			Name:      "acme/llama",
			ModelFile: map[string]types.ModelFileInfo{"Q4_0": {Name: "llama-q4_0.gguf", Downloaded: true, Size: 4}},
		})
		writeModelFile(t, s, "acme/llama", "llama-q4_0.gguf")

		if err := s.Remove("acme/llama", ""); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if _, err := os.Stat(s.ModelfilePath("acme/llama", "")); !os.IsNotExist(err) {
			t.Errorf("model dir still present, stat err = %v", err)
		}
		if locked(s, "acme/llama") {
			t.Error("lock not released after Remove")
		}
	})

	t.Run("precision not downloaded", func(t *testing.T) {
		s := newStore(t)
		seedManifest(t, s, "acme/llama", types.ModelManifest{
			Name:      "acme/llama",
			ModelFile: map[string]types.ModelFileInfo{"Q8_0": {Name: "llama-q8_0.gguf", Downloaded: false}},
		})
		// Flagged not-downloaded.
		if err := s.Remove("acme/llama", "Q8_0"); err == nil || !strings.Contains(err.Error(), "precision Q8_0 not downloaded") {
			t.Errorf("Remove not-downloaded = %v, want 'precision Q8_0 not downloaded'", err)
		}
		// Absent key takes the same path.
		if err := s.Remove("acme/llama", "Q5_K_M"); err == nil || !strings.Contains(err.Error(), "not downloaded") {
			t.Errorf("Remove absent quant = %v, want 'not downloaded'", err)
		}
	})

	t.Run("last quant removes whole dir", func(t *testing.T) {
		s := newStore(t)
		seedManifest(t, s, "acme/llama", types.ModelManifest{
			Name:      "acme/llama",
			ModelFile: map[string]types.ModelFileInfo{"Q4_0": {Name: "llama-q4_0.gguf", Downloaded: true, Size: 4}},
		})
		writeModelFile(t, s, "acme/llama", "llama-q4_0.gguf")

		if err := s.Remove("acme/llama", "Q4_0"); err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if _, err := os.Stat(s.ModelfilePath("acme/llama", "")); !os.IsNotExist(err) {
			t.Errorf("model dir still present after removing last quant, stat err = %v", err)
		}
	})

	t.Run("one of several quants flips downloaded false", func(t *testing.T) {
		s := newStore(t)
		seedManifest(t, s, "acme/llama", types.ModelManifest{
			Name: "acme/llama",
			ModelFile: map[string]types.ModelFileInfo{
				"Q4_0": {Name: "llama-q4_0.gguf", Downloaded: true, Size: 4},
				"Q8_0": {Name: "llama-q8_0.gguf", Downloaded: true, Size: 8},
			},
		})
		writeModelFile(t, s, "acme/llama", "llama-q4_0.gguf")
		writeModelFile(t, s, "acme/llama", "llama-q8_0.gguf")

		if err := s.Remove("acme/llama", "Q4_0"); err != nil {
			t.Fatalf("Remove: %v", err)
		}

		mf, err := s.readManifest("acme/llama")
		if err != nil {
			t.Fatalf("readManifest: %v", err)
		}
		q4 := mf.ModelFile["Q4_0"]
		if q4.Downloaded {
			t.Error("Q4_0 still Downloaded=true after Remove")
		}
		if q4.Name != "llama-q4_0.gguf" || q4.Size != 4 {
			t.Errorf("Q4_0 metadata not preserved: %+v", q4)
		}
		if !mf.ModelFile["Q8_0"].Downloaded {
			t.Error("Q8_0 should remain Downloaded=true")
		}
		if _, err := os.Stat(s.ModelfilePath("acme/llama", "llama-q4_0.gguf")); !os.IsNotExist(err) {
			t.Errorf("Q4_0 file not deleted, stat err = %v", err)
		}
		if _, err := os.Stat(s.ModelfilePath("acme/llama", "llama-q8_0.gguf")); err != nil {
			t.Errorf("Q8_0 file should remain: %v", err)
		}
	})
}

func TestEnsureEnoughDiskSpace(t *testing.T) {
	s := newStore(t)
	// disk.Usage stats ModelDirPath, which must exist.
	if err := os.MkdirAll(s.ModelDirPath(), 0o770); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if err := s.ensureEnoughDiskSpace(0); err != nil {
		t.Errorf("ensureEnoughDiskSpace(0) = %v, want nil", err)
	}
	// 1<<62 bytes (~4.6 EiB) exceeds any real free space; stays below the
	// int64 ceiling so the comparison can't overflow.
	if err := s.ensureEnoughDiskSpace(1 << 62); err == nil || !strings.Contains(err.Error(), "not enough disk space") {
		t.Errorf("ensureEnoughDiskSpace(1<<62) = %v, want 'not enough disk space'", err)
	}
}
