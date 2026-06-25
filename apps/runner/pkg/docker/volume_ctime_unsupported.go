// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build !darwin && !linux

package docker

import (
	"syscall"
	"time"
)

func ctimeFromStat(_ *syscall.Stat_t) (time.Time, bool) {
	return time.Time{}, false
}
