// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/daytonaio/runner/pkg/api/dto"
	"github.com/daytonaio/runner/pkg/gvisor"
	"github.com/daytonaio/runner/pkg/snapshotbundle"
	"github.com/daytonaio/runner/pkg/storage"
)

func (d *DockerClient) CreateGvisorSnapshotFromSandbox(ctx context.Context, sandboxID string, name string, store storage.SnapshotStoreClient) (*dto.SnapshotInfoResponse, error) {
	if store == nil {
		return nil, fmt.Errorf("snapshot store is required")
	}

	ctx, cancel := context.WithTimeout(ctx, time.Duration(d.backupTimeoutMin)*time.Minute)
	defer cancel()

	ct, err := d.ContainerInspect(ctx, sandboxID)
	if err != nil {
		return nil, err
	}
	if ct.State == nil || !ct.State.Running {
		return nil, fmt.Errorf("memory snapshots require a running sandbox")
	}
	if ct.Config == nil {
		return nil, fmt.Errorf("sandbox container config is missing")
	}
	if ct.ID == "" {
		return nil, fmt.Errorf("sandbox container id is missing")
	}

	baseImageRef := ct.Config.Image
	imageInfo, err := d.GetImageInfo(ctx, baseImageRef)
	if err != nil {
		return nil, fmt.Errorf("inspect base image for gVisor snapshot: %w", err)
	}

	runsc, err := gvisor.NewClientFromConfig(d.logger)
	if err != nil {
		return nil, err
	}

	manifest, err := runsc.CreateSnapshot(ctx, store, gvisor.SnapshotOptions{
		SandboxID:       ct.ID,
		Name:            name,
		BaseImageRef:    baseImageRef,
		BaseImageDigest: imageInfo.Hash,
		RuntimeConfig: snapshotbundle.RuntimeConfig{
			Entrypoint: ct.Config.Entrypoint,
			Cmd:        ct.Config.Cmd,
			Env:        envSliceToMap(ct.Config.Env),
			WorkDir:    ct.Config.WorkingDir,
			OsUser:     ct.Config.User,
		},
		Labels: ct.Config.Labels,
	})
	if err != nil {
		return nil, err
	}

	return &dto.SnapshotInfoResponse{
		Name:       manifest.Ref,
		SizeGB:     float64(manifest.SizeBytes) / (1024 * 1024 * 1024),
		Entrypoint: manifest.Config.Entrypoint,
		Cmd:        manifest.Config.Cmd,
		Hash:       manifest.Hash,
	}, nil
}

func envSliceToMap(values []string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || key == "" {
			continue
		}
		result[key] = val
	}
	return result
}
