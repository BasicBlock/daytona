/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

export const SANDBOX_LOOKUP_CACHE_TTL_MS = 10_000
export const SANDBOX_BUILD_INFO_CACHE_TTL_MS = 60_000
export const TOOLBOX_PROXY_URL_CACHE_TTL_S = 30 * 60 // 30 minutes

type SandboxLookupCacheKeyArgs = {
  returnDestroyed?: boolean
}

export function sandboxLookupCacheKeyById(args: SandboxLookupCacheKeyArgs & { sandboxId: string }): string {
  const returnDestroyed = args.returnDestroyed ? 1 : 0
  return `sandbox:lookup:by-id:returnDestroyed:${returnDestroyed}:value:${args.sandboxId}`
}

export function sandboxLookupCacheKeyByName(args: SandboxLookupCacheKeyArgs & { sandboxName: string }): string {
  const returnDestroyed = args.returnDestroyed ? 1 : 0
  return `sandbox:lookup:by-name:returnDestroyed:${returnDestroyed}:value:${args.sandboxName}`
}

export function sandboxLookupCacheKeyByAuthToken(args: { authToken: string }): string {
  return `sandbox:lookup:by-authToken:${args.authToken}`
}

export function toolboxProxyUrlCacheKey(target: string): string {
  return `toolbox-proxy-url:target:${target}`
}
