// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"context"
	"net"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
)

const daemonPort = "2280"

var daemonTCPPort = nat.Port(daemonPort + "/tcp")

func getDaemonExposedPorts() nat.PortSet {
	return nat.PortSet{
		daemonTCPPort: struct{}{},
	}
}

func GetContainerDaemonAddress(ctx context.Context, container *container.InspectResponse) string {
	if container == nil {
		return ""
	}

	if container.NetworkSettings != nil {
		for _, binding := range container.NetworkSettings.Ports[daemonTCPPort] {
			if binding.HostPort == "" {
				continue
			}

			hostIP := binding.HostIP
			if hostIP == "" || hostIP == "0.0.0.0" || hostIP == "::" {
				hostIP = "127.0.0.1"
			}

			return net.JoinHostPort(hostIP, binding.HostPort)
		}
	}

	containerIP := GetContainerIpAddress(ctx, container)
	if containerIP == "" {
		return ""
	}

	return net.JoinHostPort(containerIP, daemonPort)
}
