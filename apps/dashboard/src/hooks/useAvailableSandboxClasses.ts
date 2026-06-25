/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { useMemo } from 'react'
import { SandboxClass } from '@daytona/api-client'

const SANDBOX_CLASSES = Object.values(SandboxClass)

export function useAvailableSandboxClasses(target: string | undefined): SandboxClass[] {
  return useMemo<SandboxClass[]>(() => {
    if (!target) return []
    return SANDBOX_CLASSES
  }, [target])
}

export function useAvailableSandboxClassesForDashboard(): SandboxClass[] {
  return useMemo<SandboxClass[]>(() => SANDBOX_CLASSES, [])
}
