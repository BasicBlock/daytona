/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

package executor

import (
	"context"
	"fmt"

	apiclient "github.com/daytonaio/daytona/libs/api-client-go"
	"github.com/daytonaio/runner/pkg/api/dto"
	"github.com/daytonaio/runner/pkg/common"
)

func (e *Executor) forkSandbox(ctx context.Context, job *apiclient.Job) (any, error) {
	var payload ForkSandboxPayload
	if err := e.parsePayload(job.Payload, &payload); err != nil {
		return nil, fmt.Errorf("failed to unmarshal fork sandbox payload: %w", err)
	}

	if payload.SourceSandboxId == "" {
		return nil, fmt.Errorf("source sandbox id is required")
	}
	if payload.NewSandboxId == "" {
		return nil, fmt.Errorf("new sandbox id is required")
	}

	daemonVersion, err := e.docker.ForkSandbox(ctx, payload.SourceSandboxId, payload.NewSandboxId, payload.TargetAuthToken)
	if err != nil {
		return nil, common.FormatRecoverableError(err)
	}

	return dto.StartSandboxResponse{
		DaemonVersion: daemonVersion,
	}, nil
}
