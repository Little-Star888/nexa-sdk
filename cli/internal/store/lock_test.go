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
	"errors"
	"testing"
)

func TestLockModelEmptyName(t *testing.T) {
	s := newStore(t)
	if err := s.LockModel(""); !errors.Is(err, ErrModelNameEmpty) {
		t.Errorf("LockModel(\"\") = %v, want ErrModelNameEmpty", err)
	}
}

func TestUnlockModelEmptyName(t *testing.T) {
	s := newStore(t)
	if err := s.UnlockModel(""); err != nil {
		t.Errorf("UnlockModel(\"\") = %v, want nil", err)
	}
}

func TestLockUnlockHappyPath(t *testing.T) {
	s := newStore(t)
	if err := s.LockModel("acme/llama"); err != nil {
		t.Fatalf("LockModel: %v", err)
	}
	if !locked(s, "acme/llama") {
		t.Error("lock not recorded in modelLocks")
	}
	if err := s.UnlockModel("acme/llama"); err != nil {
		t.Fatalf("UnlockModel: %v", err)
	}
	if locked(s, "acme/llama") {
		t.Error("lock still recorded after UnlockModel")
	}
	// Re-lockable once the fd is released.
	if err := s.LockModel("acme/llama"); err != nil {
		t.Fatalf("re-LockModel: %v", err)
	}
	s.UnlockModel("acme/llama")
}

func TestLockModelDoubleLockFails(t *testing.T) {
	// Relies on flock(2) semantics (Linux/darwin/BSD build tag): the lock
	// attaches to the open file description, and LockModel opens a fresh fd
	// each call. With the first lock still held (its *Flock kept in
	// modelLocks), a second LockModel on the same name gets EWOULDBLOCK ->
	// ErrModelLocked.
	s := newStore(t)
	if err := s.LockModel("acme/llama"); err != nil {
		t.Fatalf("first LockModel: %v", err)
	}
	defer s.UnlockModel("acme/llama")

	if err := s.LockModel("acme/llama"); !errors.Is(err, ErrModelLocked) {
		t.Errorf("second LockModel = %v, want ErrModelLocked", err)
	}
}
