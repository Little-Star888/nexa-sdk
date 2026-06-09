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
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/qcom-it-nexa-ai/geniex/cli/cmd/geniex/common"
)

// A transport-layer dial failure (*net.OpError) is tagged as
// ErrServerUnreachable so PrintError can show the "is geniex serve running?"
// hint.
func TestTagServerError_DialFailure(t *testing.T) {
	opErr := &net.OpError{
		Op:  "dial",
		Net: "tcp",
		Err: errors.New("connection refused"),
	}
	got := tagServerError(opErr)
	if !errors.Is(got, common.ErrServerUnreachable) {
		t.Errorf("tagServerError(*net.OpError) not tagged as ErrServerUnreachable: %v", got)
	}
}

// A wrapped *net.OpError is still detected through errors.As.
func TestTagServerError_WrappedDialFailure(t *testing.T) {
	opErr := &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("refused")}
	wrapped := fmt.Errorf("client.Do: %w", opErr)
	got := tagServerError(wrapped)
	if !errors.Is(got, common.ErrServerUnreachable) {
		t.Errorf("wrapped *net.OpError not tagged: %v", got)
	}
}

// Non-transport errors (e.g. HTTP 4xx/5xx surfaced as plain errors) flow
// through untouched.
func TestTagServerError_PassThrough(t *testing.T) {
	orig := errors.New("400 bad request")
	got := tagServerError(orig)
	if got != orig {
		t.Errorf("tagServerError mutated a non-transport error: %v", got)
	}
	if errors.Is(got, common.ErrServerUnreachable) {
		t.Error("non-transport error wrongly tagged as ErrServerUnreachable")
	}
}

func TestTagServerError_Nil(t *testing.T) {
	if got := tagServerError(nil); got != nil {
		t.Errorf("tagServerError(nil) = %v, want nil", got)
	}
}
