/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { GpuType } from '../enums/gpu-type.enum'
import { BadRequestError } from '../../exceptions/bad-request.exception'

/**
 * Reconciles a request's GPU type preferences against an optional
 * `allowedGpuTypes` allowlist. Call once at the start of every create flow,
 * before invoking the scheduler.
 *
 * Allowlist semantics:
 *  - `null` (or `undefined`): no restriction.
 *  - `[]`: empty allowlist — all GPU types are blocked.
 *  - non-empty array: only the listed types are permitted.
 *
 * @returns Effective preference list to pass to the scheduler, or `undefined`
 *   when no GPU type filter should be applied.
 * @throws {BadRequestError} When all GPU types are blocked, or none of
 *   the requested preferences intersect with the allowlist.
 */
export function resolveGpuTypePreferences(
  gpu: number,
  gpuTypePreferences: GpuType[] | undefined,
  allowedGpuTypes: GpuType[] | null | undefined,
): GpuType[] | undefined {
  if (gpu <= 0) return undefined

  if (allowedGpuTypes == null) {
    return gpuTypePreferences && gpuTypePreferences.length > 0 ? gpuTypePreferences : undefined
  }

  if (allowedGpuTypes.length === 0) {
    throw new BadRequestError('No GPU types are allowed for this runner target.')
  }

  if (!gpuTypePreferences || gpuTypePreferences.length === 0) {
    return allowedGpuTypes
  }

  const permitted = gpuTypePreferences.filter((t) => allowedGpuTypes.includes(t))
  if (permitted.length === 0) {
    throw new BadRequestError(
      `Requested GPU type(s) ${gpuTypePreferences.join(', ')} not permitted for this runner target. Allowed: ${allowedGpuTypes.join(', ')}.`,
    )
  }
  return permitted
}
