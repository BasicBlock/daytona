/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

package executor

type StartSandboxPayload struct {
	AuthToken *string           `json:"authToken,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

type SnapshotSandboxPayload struct {
	SandboxId string `json:"sandboxId"`
	Name      string `json:"name"`
}

type ForkSandboxPayload struct {
	SourceSandboxId string `json:"sourceSandboxId"`
	NewSandboxId    string `json:"newSandboxId"`
	TargetAuthToken string `json:"targetAuthToken,omitempty"`
}
