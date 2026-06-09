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

package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/qcom-it-nexa-ai/geniex/cli/internal/config"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestCORS_SetsHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	CORS(c)

	h := w.Header()
	checks := map[string]string{
		"Access-Control-Allow-Origin":      config.Get().Origins,
		"Access-Control-Allow-Methods":     "OPTIONS, GET, POST",
		"Access-Control-Allow-Headers":     "Content-Type, GenieX-KeepCache",
		"Access-Control-Allow-Credentials": "true",
		"Access-Control-Max-Age":           "86400",
	}
	for k, want := range checks {
		if got := h.Get(k); got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}
}

// A non-OPTIONS request must call the next handler and not abort.
func TestCORS_PassesThroughNonOptions(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	CORS(c)

	if c.IsAborted() {
		t.Error("CORS aborted a GET request")
	}
}

// An OPTIONS preflight must short-circuit with 204 and abort the chain.
func TestCORS_OptionsShortCircuits(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodOptions, "/", nil)

	CORS(c)

	if !c.IsAborted() {
		t.Error("CORS did not abort an OPTIONS request")
	}
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want %d", w.Code, http.StatusNoContent)
	}
}

// CORS in a full engine must let the route handler run and emit its body.
func TestCORS_InEngineRunsHandler(t *testing.T) {
	r := gin.New()
	r.Use(CORS)
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/ping", nil))

	if w.Code != http.StatusOK || w.Body.String() != "pong" {
		t.Errorf("got %d %q, want 200 pong", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != config.Get().Origins {
		t.Errorf("Access-Control-Allow-Origin = %q, want %q", got, config.Get().Origins)
	}
}

// GIL must serialize concurrent handlers: with the middleware in place no two
// handler bodies overlap, so the observed max concurrency stays at 1.
func TestGIL_SerializesHandlers(t *testing.T) {
	r := gin.New()
	r.Use(GIL)

	var inFlight, maxSeen int32
	r.GET("/work", func(c *gin.Context) {
		n := atomic.AddInt32(&inFlight, 1)
		for {
			old := atomic.LoadInt32(&maxSeen)
			if n <= old || atomic.CompareAndSwapInt32(&maxSeen, old, n) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt32(&inFlight, -1)
		c.Status(http.StatusOK)
	})

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/work", nil))
		}()
	}
	wg.Wait()

	if maxSeen != 1 {
		t.Errorf("max concurrent handlers = %d, want 1 (GIL should serialize)", maxSeen)
	}
}
