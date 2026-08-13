// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package middleware

import (
	"sync"

	"github.com/gin-gonic/gin"
)

// GILock serializes all API requests. The keep-alive sweeper shares it
// (service.sweep): a cached model is only destroyed while no request is in
// flight, so it can never be freed mid-generation (#1322).
var GILock sync.Mutex

func GIL(c *gin.Context) {
	// Block and wait for lock instead of immediately failing
	// This prevents 429 errors when requests queue up briefly
	GILock.Lock()
	defer GILock.Unlock()

	c.Next()
}
