// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"context"
	"fmt"
)

func (d *DockerClient) ForkSandbox(ctx context.Context, sourceSandboxID string, newSandboxID string, targetAuthToken string) (string, error) {
	if d.runscRuntime == nil {
		return "", fmt.Errorf("raw runsc sandbox lifecycle backend is not initialized")
	}
	if !d.runscRuntime.Exists(sourceSandboxID) {
		return "", fmt.Errorf("source sandbox %s is not managed by the raw runsc lifecycle backend", sourceSandboxID)
	}
	return d.runscRuntime.Fork(ctx, sourceSandboxID, newSandboxID, targetAuthToken)
}
