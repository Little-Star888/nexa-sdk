// Copyright 2024-2026 Qualcomm Technologies, Inc. and/or its subsidiaries.
// SPDX-License-Identifier: BSD-3-Clause

package service

import (
	"log/slog"

	"github.com/qualcomm/GenieX/cli/internal/store"
)

// Neither config.json nor the host probe changes while the process lives.
var hostChipset string

func Init() {
	hostChipset = store.Get().ResolveChipset(true)
	slog.Debug("resolved host chipset", "chipset", hostChipset)
	keepAlive.start()
}

// Chipset returns the host chipset resolved at startup, "" when unknown.
func Chipset() string {
	return hostChipset
}

func DeInit() {
	keepAlive.stop()
}
