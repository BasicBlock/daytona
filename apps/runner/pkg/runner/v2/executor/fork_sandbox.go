/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

package executor

import (
	"context"
	"fmt"

	apiclient "github.com/daytonaio/daytona/libs/api-client-go"
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

	return nil, fmt.Errorf("gVisor sandbox fork requires the raw runsc sandbox lifecycle backend; Docker restore/checkpoint fallback is intentionally disabled")
}
