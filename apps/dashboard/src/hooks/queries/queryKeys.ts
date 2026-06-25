/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { SnapshotQueryParams } from './useSnapshotsQuery'
import type { SandboxQueryParams } from './useSandboxesQuery'

export const queryKeys = {
  config: {
    all: ['config'] as const,
  },
  snapshots: {
    all: ['snapshots'] as const,
    detail: (snapshotId: string) => [...queryKeys.snapshots.all, snapshotId, 'detail'] as const,
    list: (params?: SnapshotQueryParams) => {
      const base = [...queryKeys.snapshots.all, 'list'] as const
      if (!params) return base
      return [
        ...base,
        {
          page: params.page,
          pageSize: params.pageSize,
          ...(params.filters && { filters: params.filters }),
          ...(params.sorting && { sorting: params.sorting }),
        },
      ] as const
    },
  },
  registries: {
    all: ['registries'] as const,
    list: () => [...queryKeys.registries.all, 'list'] as const,
  },
  targets: {
    all: ['targets'] as const,
    shared: () => [...queryKeys.targets.all, 'shared'] as const,
    available: () => [...queryKeys.targets.all, 'available'] as const,
  },
  runners: {
    all: ['runners'] as const,
    list: (target?: string) => {
      const base = [...queryKeys.runners.all, 'list'] as const
      if (!target) return base

      return [...base, { target }] as const
    },
  },
  volumes: {
    all: ['volumes'] as const,
    list: () => [...queryKeys.volumes.all, 'list'] as const,
  },
  sandboxes: {
    all: ['sandboxes'] as const,
    list: (params?: SandboxQueryParams) => {
      const base = [...queryKeys.sandboxes.all, 'list'] as const
      if (!params) return base

      return [
        ...base,
        {
          cursor: params.cursor,
          limit: params.limit,
          ...(params.filters && {
            filters: {
              ...params.filters,
              createdAtAfter: params.filters.createdAtAfter?.toISOString(),
              createdAtBefore: params.filters.createdAtBefore?.toISOString(),
              lastEventAfter: params.filters.lastEventAfter?.toISOString(),
              lastEventBefore: params.filters.lastEventBefore?.toISOString(),
            },
          }),
          ...(params.sorting && { sorting: params.sorting }),
        },
      ] as const
    },
    detail: (sandboxId: string) => [...queryKeys.sandboxes.all, sandboxId, 'detail'] as const,
    terminalSession: (sandboxId: string) => [...queryKeys.sandboxes.all, sandboxId, 'terminal-session'] as const,
    vncInitialStatus: (sandboxId: string) => [...queryKeys.sandboxes.all, sandboxId, 'vnc-initial-status'] as const,
    vncPollStatus: (sandboxId: string) => [...queryKeys.sandboxes.all, sandboxId, 'vnc-poll-status'] as const,
    vncSession: (sandboxId: string) => [...queryKeys.sandboxes.all, sandboxId, 'vnc-session'] as const,
  },
  telemetry: {
    all: ['telemetry'] as const,
    logs: (sandboxId: string, params: object) => [...queryKeys.telemetry.all, sandboxId, 'logs', params] as const,
    traces: (sandboxId: string, params: object) => [...queryKeys.telemetry.all, sandboxId, 'traces', params] as const,
    metrics: (sandboxId: string, params: object) => [...queryKeys.telemetry.all, sandboxId, 'metrics', params] as const,
    traceSpans: (sandboxId: string, traceId: string) =>
      [...queryKeys.telemetry.all, sandboxId, 'traces', traceId] as const,
  },
  sandbox: {
    all: ['sandbox'] as const,
    session: (scope: string) => [...queryKeys.sandbox.all, scope] as const,
    currentId: (scope: string) => [...queryKeys.sandbox.all, scope, 'current-id'] as const,
    instance: (scope: string, id: string) => [...queryKeys.sandbox.all, scope, id] as const,
    terminalUrl: (scope: string, id: string) => [...queryKeys.sandbox.all, scope, id, 'terminal-url'] as const,
    vncStatus: (scope: string, id: string) => [...queryKeys.sandbox.all, scope, id, 'vnc-status'] as const,
    vncUrl: (scope: string, id: string) => [...queryKeys.sandbox.all, scope, id, 'vnc-url'] as const,
  },
} as const
