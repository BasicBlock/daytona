// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build !linux

package port

func listeningTCPPorts() (map[string]bool, error) {
	return map[string]bool{}, nil
}
