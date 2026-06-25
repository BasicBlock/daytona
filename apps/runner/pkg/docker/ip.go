// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"context"
	"sort"

	"github.com/docker/docker/api/types/container"
)

// GetContainerIpAddress returns the IP address of the container on its primary
// reachable network.
//
// Returns an empty string when no known network is attached.
func GetContainerIpAddress(ctx context.Context, container *container.InspectResponse) string {
	if container == nil || container.NetworkSettings == nil || container.NetworkSettings.Networks == nil {
		return ""
	}

	if networkSettings, ok := container.NetworkSettings.Networks[RUNNER_BRIDGE_NETWORK_NAME]; ok && networkSettings != nil && networkSettings.IPAddress != "" {
		return networkSettings.IPAddress
	}

	if networkSettings, ok := container.NetworkSettings.Networks["bridge"]; ok && networkSettings != nil && networkSettings.IPAddress != "" {
		return networkSettings.IPAddress
	}

	networkNames := make([]string, 0, len(container.NetworkSettings.Networks))
	for name := range container.NetworkSettings.Networks {
		networkNames = append(networkNames, name)
	}
	sort.Strings(networkNames)
	for _, name := range networkNames {
		networkSettings := container.NetworkSettings.Networks[name]
		if networkSettings != nil && networkSettings.IPAddress != "" {
			return networkSettings.IPAddress
		}
	}

	return ""
}
