/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Target, TargetType } from '@daytona/api-client'

export const EMPTY_TARGETS: Target[] = []

export const filterCustomTargets = (targets: Target[]) => {
  return targets.filter((target) => target.targetType === TargetType.CUSTOM)
}

export const createTargetNameGetter = (...targetLists: Target[][]) => {
  const targetNameById = new Map<string, string>()

  for (const targets of targetLists) {
    for (const target of targets) {
      if (!targetNameById.has(target.id)) {
        targetNameById.set(target.id, target.name)
      }
    }
  }

  return (target: string) => targetNameById.get(target)
}
