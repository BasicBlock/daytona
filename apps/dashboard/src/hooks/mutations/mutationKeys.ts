/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export const mutationKeys = {
  sandboxes: {
    all: ['sandboxes'] as const,
    start: () => [...mutationKeys.sandboxes.all, 'start'] as const,
    stop: () => [...mutationKeys.sandboxes.all, 'stop'] as const,
    archive: () => [...mutationKeys.sandboxes.all, 'archive'] as const,
    recover: () => [...mutationKeys.sandboxes.all, 'recover'] as const,
    remove: () => [...mutationKeys.sandboxes.all, 'remove'] as const,
    fork: () => [...mutationKeys.sandboxes.all, 'fork'] as const,
    createSnapshot: () => [...mutationKeys.sandboxes.all, 'create-snapshot'] as const,
    vnc: () => [...mutationKeys.sandboxes.all, 'vnc'] as const,
    screenRecordings: () => [...mutationKeys.sandboxes.all, 'screen-recordings'] as const,
  },
  runners: {
    all: ['runners'] as const,
    create: () => [...mutationKeys.runners.all, 'create'] as const,
    updateScheduling: () => [...mutationKeys.runners.all, 'update-scheduling'] as const,
    remove: () => [...mutationKeys.runners.all, 'remove'] as const,
  },
} as const
