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
	"strings"
	"testing"
)

func TestConfigSetThenGet(t *testing.T) {
	s := newStore(t)
	if err := s.ConfigSet(ConfigKeyDevice, "npu"); err != nil {
		t.Fatalf("ConfigSet: %v", err)
	}
	got, ok, err := s.ConfigGet(ConfigKeyDevice)
	if err != nil {
		t.Fatalf("ConfigGet: %v", err)
	}
	if !ok || got != "npu" {
		t.Errorf("ConfigGet = (%q, %v), want (npu, true)", got, ok)
	}
}

func TestConfigGetMissingKey(t *testing.T) {
	s := newStore(t)
	got, ok, err := s.ConfigGet(ConfigKeyDevice)
	if err != nil {
		t.Fatalf("ConfigGet: %v", err)
	}
	if ok || got != "" {
		t.Errorf("ConfigGet on empty store = (%q, %v), want (\"\", false)", got, ok)
	}
}

func TestConfigEmptyValueDeletesKey(t *testing.T) {
	s := newStore(t)
	if err := s.ConfigSet(ConfigKeyDevice, "npu"); err != nil {
		t.Fatalf("ConfigSet: %v", err)
	}
	if err := s.ConfigSet(ConfigKeyDevice, ""); err != nil {
		t.Fatalf("ConfigSet empty: %v", err)
	}
	if _, ok, _ := s.ConfigGet(ConfigKeyDevice); ok {
		t.Error("key still present after setting empty value")
	}
	list, err := s.ConfigList()
	if err != nil {
		t.Fatalf("ConfigList: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ConfigList = %v, want empty", list)
	}
}

func TestConfigOverwrite(t *testing.T) {
	s := newStore(t)
	if err := s.ConfigSet(ConfigKeyDevice, "npu"); err != nil {
		t.Fatalf("ConfigSet: %v", err)
	}
	if err := s.ConfigSet(ConfigKeyDevice, "cpu"); err != nil {
		t.Fatalf("ConfigSet overwrite: %v", err)
	}
	got, _, _ := s.ConfigGet(ConfigKeyDevice)
	if got != "cpu" {
		t.Errorf("ConfigGet = %q, want cpu", got)
	}
}

func TestConfigListSnapshot(t *testing.T) {
	s := newStore(t)
	if err := s.ConfigSet(ConfigKeyDevice, "npu"); err != nil {
		t.Fatalf("ConfigSet device: %v", err)
	}
	if err := s.ConfigSet("foo", "bar"); err != nil {
		t.Fatalf("ConfigSet foo: %v", err)
	}
	list, err := s.ConfigList()
	if err != nil {
		t.Fatalf("ConfigList: %v", err)
	}
	if len(list) != 2 || list[ConfigKeyDevice] != "npu" || list["foo"] != "bar" {
		t.Errorf("ConfigList = %v, want {device:npu, foo:bar}", list)
	}
}

func TestLoadConfigMissingFileReturnsEmpty(t *testing.T) {
	s := newStore(t)
	cfg, err := s.loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg == nil {
		t.Fatal("loadConfig returned nil map, want empty map")
	}
	if len(cfg) != 0 {
		t.Errorf("loadConfig = %v, want empty", cfg)
	}
}

func TestSaveConfigAtomicNoTempLeftover(t *testing.T) {
	s := newStore(t)
	if err := s.ConfigSet(ConfigKeyDevice, "npu"); err != nil {
		t.Fatalf("ConfigSet: %v", err)
	}
	// Config file exists and round-trips.
	if _, err := os.Stat(s.configPath()); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
	cfg, err := s.loadConfig()
	if err != nil || cfg[ConfigKeyDevice] != "npu" {
		t.Fatalf("round-trip = (%v, %v), want device=npu", cfg, err)
	}
	// The atomic write (temp + rename) leaves no .tmp behind.
	entries, err := os.ReadDir(s.home)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".config-") && strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}
