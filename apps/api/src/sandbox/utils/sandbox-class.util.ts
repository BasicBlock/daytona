/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */
import { SandboxClass } from '../enums/sandbox-class.enum'

export function getRunnerSandboxClass(sandboxClass: SandboxClass): SandboxClass {
  return sandboxClass
}

export function isGcsSnapshotRef(ref?: string | null): boolean {
  return !!ref && ref.startsWith('gcs://')
}

export function isRegistrySnapshotRef(sandboxClass: SandboxClass, ref?: string | null): boolean {
  void sandboxClass
  return !isGcsSnapshotRef(ref)
}
