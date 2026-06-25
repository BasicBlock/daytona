// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"context"
	"fmt"

	"github.com/daytonaio/common-go/pkg/timer"
	"github.com/daytonaio/runner/pkg/api/dto"
)

func (d *DockerClient) Create(ctx context.Context, sandboxDto dto.CreateSandboxDTO) (string, string, error) {
	defer timer.Timer()()

	if d.runscRuntime == nil {
		return "", "", fmt.Errorf("raw runsc sandbox lifecycle backend is not initialized")
	}

	return d.runscRuntime.Create(ctx, sandboxDto)
}
