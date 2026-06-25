/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export const SandboxEvents = {
  ARCHIVED: 'sandbox.archived',
  STATE_UPDATED: 'sandbox.state.updated',
  DESIRED_STATE_UPDATED: 'sandbox.desired-state.updated',
  CREATED: 'sandbox.created',
  STARTED: 'sandbox.started',
  STOPPED: 'sandbox.stopped',
  DESTROYED: 'sandbox.destroyed',
  BACKUP_CREATED: 'sandbox.backup.created',
} as const
