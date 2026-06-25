/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Injectable, Logger } from '@nestjs/common'
import { DataSource } from 'typeorm'
import {
  sandboxLookupCacheKeyByAuthToken,
  sandboxLookupCacheKeyById,
  sandboxLookupCacheKeyByName,
} from '../utils/sandbox-lookup-cache.util'

type InvalidateSandboxLookupCacheArgs =
  | {
      sandboxId: string
      name: string
      previousName?: string | null
    }
  | {
      authToken: string
    }

@Injectable()
export class SandboxLookupCacheInvalidationService {
  private readonly logger = new Logger(SandboxLookupCacheInvalidationService.name)

  constructor(private readonly dataSource: DataSource) {}

  invalidate(args: InvalidateSandboxLookupCacheArgs): void {
    const cache = this.dataSource.queryResultCache
    if (!cache) {
      return
    }

    if ('authToken' in args) {
      const tokenPrefix = `${args.authToken.slice(0, 6)}...`
      cache
        .remove([sandboxLookupCacheKeyByAuthToken({ authToken: args.authToken })])
        .then(() => this.logger.debug(`Invalidated sandbox lookup cache for authToken ${tokenPrefix}`))
        .catch((error) =>
          this.logger.warn(
            `Failed to invalidate sandbox lookup cache for authToken ${tokenPrefix}: ${error instanceof Error ? error.message : String(error)}`,
          ),
        )
      return
    }

    const names = Array.from(
      new Set([args.name, args.previousName].filter((n): n is string => Boolean(n && n.trim().length > 0))),
    )

    const cacheIds: string[] = []
    for (const returnDestroyed of [false, true]) {
      cacheIds.push(
        sandboxLookupCacheKeyById({
          returnDestroyed,
          sandboxId: args.sandboxId,
        }),
      )
      for (const sandboxName of names) {
        cacheIds.push(
          sandboxLookupCacheKeyByName({
            returnDestroyed,
            sandboxName,
          }),
        )
      }
    }

    if (cacheIds.length === 0) {
      return
    }

    cache
      .remove(cacheIds)
      .then(() => this.logger.debug(`Invalidated sandbox lookup cache for ${args.sandboxId}`))
      .catch((error) =>
        this.logger.warn(
          `Failed to invalidate sandbox lookup cache for ${args.sandboxId}: ${error instanceof Error ? error.message : String(error)}`,
        ),
      )
  }
}
