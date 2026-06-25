// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build linux

package docker

import (
	"syscall"
	"time"
)

func ctimeFromStat(stat *syscall.Stat_t) (time.Time, bool) {
	if stat == nil {
		return time.Time{}, false
	}
	return time.Unix(stat.Ctim.Sec, stat.Ctim.Nsec), true
}
