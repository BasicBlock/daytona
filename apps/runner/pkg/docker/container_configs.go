// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strconv"
	"strings"

	"github.com/daytonaio/runner/cmd/runner/config"
	"github.com/daytonaio/runner/pkg/api/dto"
	"github.com/daytonaio/runner/pkg/common"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/go-connections/nat"

	"github.com/docker/docker/api/types/container"
)

// Fixed sandbox slice applied to every GPU sandbox regardless of host size.
// Keeping these constant across runners makes user-visible GPU sandbox
// capacity uniform on heterogeneous GPU fleets.
const (
	gpuSandboxCPUCores  int64 = 16
	gpuSandboxMemoryGiB int64 = 256
	gpuSandboxDiskGiB   int64 = 512
	runscRuntime              = "runsc"
)

const sandboxAuthTokenLabel = "daytona.auth_token"

func GetContainerAuthToken(c *container.InspectResponse) string {
	if c == nil || c.Config == nil || c.Config.Labels == nil {
		return ""
	}
	return c.Config.Labels[sandboxAuthTokenLabel]
}

func (d *DockerClient) getContainerConfigs(sandboxDto dto.CreateSandboxDTO, image *image.InspectResponse, volumeMountPathBinds []string, gpuIndex *int, extraHosts []string) (*container.Config, *container.HostConfig, *network.NetworkingConfig, error) {
	containerConfig, err := d.getContainerCreateConfig(sandboxDto, image, gpuIndex)
	if err != nil {
		return nil, nil, nil, err
	}

	hostConfig, err := d.getContainerHostConfig(sandboxDto, volumeMountPathBinds, gpuIndex, extraHosts)
	if err != nil {
		return nil, nil, nil, err
	}

	networkingConfig := d.getContainerNetworkingConfig(sandboxDto)

	return containerConfig, hostConfig, networkingConfig, nil
}

func (d *DockerClient) getContainerCreateConfig(sandboxDto dto.CreateSandboxDTO, image *image.InspectResponse, gpuIndex *int) (*container.Config, error) {
	if image == nil {
		return nil, fmt.Errorf("image not found for sandbox: %s", sandboxDto.Id)
	}

	envVars := []string{
		"DAYTONA_SANDBOX_ID=" + sandboxDto.Id,
		"DAYTONA_SANDBOX_SNAPSHOT=" + sandboxDto.Snapshot,
		"DAYTONA_SANDBOX_USER=" + sandboxDto.OsUser,
	}

	// GPU sandboxes run non-privileged so CDI's per-device cgroup rules
	// actually take effect. CDI already restricts the container to the one
	// allocated physical GPU (see DeviceRequests below), and Linux/CUDA
	// renumber the exposed devices starting at 0 - so from inside the
	// container the GPU is always index 0 regardless of which host slot
	// was allocated. Hard-code the env vars to "0" so CUDA/userspace tools
	// don't try to address a host-side index that doesn't exist in the
	// container's view (which would break e.g. cudaSetDevice while letting
	// nvidia-smi work).
	if gpuIndex != nil {
		envVars = append(envVars,
			"NVIDIA_VISIBLE_DEVICES=0",
			"CUDA_VISIBLE_DEVICES=0",
		)
	}

	for key, value := range sandboxDto.Env {
		envVars = append(envVars, fmt.Sprintf("%s=%s", key, value))
	}

	if sandboxDto.OtelEndpoint != nil && *sandboxDto.OtelEndpoint != "" {
		envVars = append(envVars, "DAYTONA_OTEL_ENDPOINT="+*sandboxDto.OtelEndpoint)
	}

	labels := make(map[string]string)
	if sandboxDto.Name != "" {
		labels[sandboxNameLabel] = sandboxDto.Name
	}
	if sandboxDto.AuthToken != nil && *sandboxDto.AuthToken != "" {
		labels[sandboxAuthTokenLabel] = *sandboxDto.AuthToken
	}
	if len(sandboxDto.Volumes) > 0 {
		volumeMountPaths := make([]string, len(sandboxDto.Volumes))
		for i, v := range sandboxDto.Volumes {
			volumeMountPaths[i] = v.MountPath
		}
		labels["daytona.volume_mount_paths"] = strings.Join(volumeMountPaths, ",")
	}
	if gpuIndex != nil {
		labels[GpuIndexLabel] = strconv.Itoa(*gpuIndex)
	}

	workingDir := ""
	cmd := []string{}
	entrypoint := sandboxDto.Entrypoint
	if !d.useSnapshotEntrypoint {
		if image.Config.WorkingDir != "" {
			workingDir = image.Config.WorkingDir
		}

		// If workingDir is empty, append flag env var to envVars
		if workingDir == "" {
			envVars = append(envVars, "DAYTONA_USER_HOME_AS_WORKDIR=true")
		}

		entrypoint = []string{common.DAEMON_PATH}

		if len(sandboxDto.Entrypoint) != 0 {
			cmd = append(cmd, sandboxDto.Entrypoint...)
		} else {
			if slices.Equal(image.Config.Entrypoint, strslice.StrSlice{common.DAEMON_PATH}) {
				cmd = append(cmd, image.Config.Cmd...)
			} else {
				cmd = append(cmd, image.Config.Entrypoint...)
			}
		}
	}

	return &container.Config{
		Hostname:     sandboxDto.Id,
		Image:        sandboxDto.Snapshot,
		WorkingDir:   workingDir,
		Env:          envVars,
		Entrypoint:   entrypoint,
		Cmd:          cmd,
		Labels:       labels,
		AttachStdout: true,
		AttachStderr: true,
		ExposedPorts: getDaemonExposedPorts(),
	}, nil
}

func (d *DockerClient) getContainerHostConfig(sandboxDto dto.CreateSandboxDTO, volumeMountPathBinds []string, gpuIndex *int, extraHosts []string) (*container.HostConfig, error) {
	var binds []string

	binds = append(binds, fmt.Sprintf("%s:%s:ro", d.daemonPath, common.DAEMON_PATH))

	// Mount the plugin if available
	if d.computerUsePluginPath != "" {
		binds = append(binds, fmt.Sprintf("%s:/usr/local/lib/daytona-computer-use:ro", d.computerUsePluginPath))
	}

	if len(volumeMountPathBinds) > 0 {
		binds = append(binds, volumeMountPathBinds...)
	}

	hostConfig := &container.HostConfig{
		// Privileged mode exposes every /dev/nvidia* node and bypasses the
		// CDI cgroup rules, so GPU sandboxes have to opt out to keep their
		// allocated card isolated. Non-GPU sandboxes still need privileged
		// for their current workloads.
		Privileged: gpuIndex == nil,
		Binds:      binds,
		DNS:        sandboxDNSServers(),
	}
	if runtime.GOOS != "linux" {
		hostConfig.PortBindings = nat.PortMap{
			daemonTCPPort: []nat.PortBinding{{HostIP: "127.0.0.1"}},
		}
	}

	if sandboxDto.OtelEndpoint != nil && strings.Contains(*sandboxDto.OtelEndpoint, "host.docker.internal") {
		extraHosts = append(extraHosts, "host.docker.internal:host-gateway")
	}
	if len(extraHosts) > 0 {
		hostConfig.ExtraHosts = dedupeStrings(extraHosts)
	}

	// GPU sandboxes ignore the API-requested resources and instead get a
	// uniform slice that is identical on every GPU runner regardless of
	// host size. This keeps user-visible sandbox capacity consistent
	// across heterogeneous GPU fleets (e.g. H100 NVL vs H100 SXM5 hosts).
	cpuQuota := sandboxDto.CpuQuota
	memoryQuotaGiB := sandboxDto.MemoryQuota
	storageQuotaGiB := sandboxDto.StorageQuota
	if gpuIndex != nil {
		cpuQuota = gpuSandboxCPUCores
		memoryQuotaGiB = gpuSandboxMemoryGiB
		storageQuotaGiB = gpuSandboxDiskGiB
	}

	if !d.resourceLimitsDisabled {
		hostConfig.Resources = container.Resources{
			CPUPeriod:  100000,
			CPUQuota:   cpuQuota * 100000,
			Memory:     common.GBToBytes(float64(memoryQuotaGiB)),
			MemorySwap: common.GBToBytes(float64(memoryQuotaGiB)),
		}
	}

	hostConfig.Runtime = runscRuntime

	if !d.resourceLimitsDisabled && d.filesystem == "xfs" {
		hostConfig.StorageOpt = map[string]string{
			"size": fmt.Sprintf("%dG", storageQuotaGiB),
		}
	}

	if d.gpuEnabled && gpuIndex != nil {
		hostConfig.DeviceRequests = []container.DeviceRequest{{
			Driver:    "cdi",
			DeviceIDs: []string{fmt.Sprintf("nvidia.com/gpu=%d", *gpuIndex)},
		}}
	}

	return hostConfig, nil
}

func sandboxDNSServers() []string {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return []string{"1.1.1.1", "8.8.8.8"}
	}

	servers := make([]string, 0)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "nameserver" {
			continue
		}
		if fields[1] == "127.0.0.11" {
			continue
		}
		servers = append(servers, fields[1])
	}
	if len(servers) == 0 {
		return []string{"1.1.1.1", "8.8.8.8"}
	}
	return dedupeStrings(servers)
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func (d *DockerClient) getContainerNetworkingConfig(sandboxDto dto.CreateSandboxDTO) *network.NetworkingConfig {
	sandboxNetwork := config.GetSandboxNetwork()
	var networkingConfig *network.NetworkingConfig
	if sandboxNetwork != "" {
		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				sandboxNetwork: {},
			},
		}
	}

	if !d.interSandboxNetworkEnabled {
		if networkingConfig == nil {
			networkingConfig = &network.NetworkingConfig{
				EndpointsConfig: map[string]*network.EndpointSettings{},
			}
		}
		networkingConfig.EndpointsConfig[RUNNER_BRIDGE_NETWORK_NAME] = &network.EndpointSettings{}
	}

	if networkingConfig == nil {
		networkingConfig = &network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{},
		}
	}

	if sandboxNetwork == "" && d.interSandboxNetworkEnabled {
		networkingConfig.EndpointsConfig["bridge"] = &network.EndpointSettings{}
	}

	if sandboxDto.LinkedSandboxId != nil && *sandboxDto.LinkedSandboxId != "" {
		linkOwnerId := *sandboxDto.LinkedSandboxId
		networkingConfig.EndpointsConfig[linkNetworkName(linkOwnerId)] = &network.EndpointSettings{
			Aliases: networkAliasesForSandbox(sandboxDto.Id, sandboxDto.Name),
		}
	}

	return networkingConfig
}
