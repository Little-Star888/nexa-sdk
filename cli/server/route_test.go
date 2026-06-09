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

package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func newTestEngine() *gin.Engine {
	r := gin.New()
	RegisterRoot(r)
	RegisterSwagger(r)
	RegisterAPIv1(r)
	return r
}

func do(r *gin.Engine, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

// RegisterRoot wires "/" to redirect to the docs UI.
func TestRegisterRoot_RedirectsToDocs(t *testing.T) {
	w := do(newTestEngine(), http.MethodGet, "/")
	if w.Code != http.StatusFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusFound)
	}
	if loc := w.Header().Get("Location"); loc != "/docs/ui/" {
		t.Errorf("Location = %q, want /docs/ui/", loc)
	}
}

// RegisterRoot also installs CORS at the root, so even a plain "/" carries the
// CORS headers (the allow-methods header is a fixed constant in the middleware).
func TestRegisterRoot_AppliesCORS(t *testing.T) {
	w := do(newTestEngine(), http.MethodGet, "/")
	if got := w.Header().Get("Access-Control-Allow-Methods"); got != "OPTIONS, GET, POST" {
		t.Errorf("CORS allow-methods = %q, want the middleware constant", got)
	}
}

// The /v1/ ping endpoint reports liveness.
func TestRegisterAPIv1_Ping(t *testing.T) {
	w := do(newTestEngine(), http.MethodGet, "/v1/")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "GenieX-CLI is running" {
		t.Errorf("body = %q", w.Body.String())
	}
}

// The legacy /v1/completions endpoint is permanently retired (410 Gone).
func TestRegisterAPIv1_LegacyCompletionsGone(t *testing.T) {
	w := do(newTestEngine(), http.MethodPost, "/v1/completions")
	if w.Code != http.StatusGone {
		t.Errorf("status = %d, want %d", w.Code, http.StatusGone)
	}
}

// The docs swagger spec is served as YAML.
func TestRegisterSwagger_ServesYAML(t *testing.T) {
	w := do(newTestEngine(), http.MethodGet, "/docs/swagger.yaml")
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}
