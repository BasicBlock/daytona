/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export type CreateRunnerV2InternalDto = {
  target: string
  name: string
  apiVersion: '2'
  appVersion?: string
  tags?: string[]
}

export type CreateRunnerInternalDto = CreateRunnerV2InternalDto
