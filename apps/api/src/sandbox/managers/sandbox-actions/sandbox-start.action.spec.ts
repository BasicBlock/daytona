/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { RegistryType } from '../../../docker-registry/enums/registry-type.enum'
import type { DockerRegistry } from '../../../docker-registry/entities/docker-registry.entity'
import type { Runner } from '../../entities/runner.entity'
import type { Snapshot } from '../../entities/snapshot.entity'
import { SandboxClass } from '../../enums/sandbox-class.enum'
import { SandboxStartAction } from './sandbox-start.action'

describe('SandboxStartAction', () => {
  function createAction(adapter: { pullSnapshot: jest.Mock; getSnapshotInfo: jest.Mock }) {
    const dockerRegistryService = {
      findSourceRegistryBySnapshotImageName: jest.fn(),
    }

    const action = new SandboxStartAction(
      {} as never,
      { create: jest.fn().mockResolvedValue(adapter) } as never,
      {} as never,
      {} as never,
      dockerRegistryService as never,
      { get: jest.fn(), getOrThrow: jest.fn() } as never,
      {} as never,
      {} as never,
      {} as never,
    )

    return { action, dockerRegistryService }
  }

  const runner = {
    id: 'runner-1',
    target: 'default',
  } as Runner

  it('passes source registry credentials when pulling a registry-backed snapshot', async () => {
    const adapter = {
      pullSnapshot: jest.fn().mockResolvedValue(undefined),
      getSnapshotInfo: jest.fn().mockResolvedValue({}),
    }
    const { action, dockerRegistryService } = createAction(adapter)
    const registry = {
      id: 'registry-1',
      name: 'GAR',
      url: 'https://us-central1-docker.pkg.dev',
      username: '_json_key',
      password: 'secret',
      project: 'basicblock',
      registryType: RegistryType.CUSTOM,
    } as DockerRegistry
    const snapshot = {
      ref: 'us-central1-docker.pkg.dev/basicblock/docker-images/devcontainer:f7cda6b',
      sandboxClass: SandboxClass.LINUX_VM,
    } as Snapshot

    dockerRegistryService.findSourceRegistryBySnapshotImageName.mockResolvedValue(registry)

    await action.pullSnapshotToRunner(snapshot, runner)

    expect(dockerRegistryService.findSourceRegistryBySnapshotImageName).toHaveBeenCalledWith(
      snapshot.ref,
      runner.target,
    )
    expect(adapter.pullSnapshot).toHaveBeenCalledWith(snapshot.ref, registry)
  })

  it('does not resolve registry credentials for GCS snapshot refs', async () => {
    const adapter = {
      pullSnapshot: jest.fn().mockResolvedValue(undefined),
      getSnapshotInfo: jest.fn().mockResolvedValue({}),
    }
    const { action, dockerRegistryService } = createAction(adapter)
    const snapshot = {
      ref: 'gcs://daytona-snapshots/base/snapshot.json',
      sandboxClass: SandboxClass.LINUX_VM,
    } as Snapshot

    await action.pullSnapshotToRunner(snapshot, runner)

    expect(dockerRegistryService.findSourceRegistryBySnapshotImageName).not.toHaveBeenCalled()
    expect(adapter.pullSnapshot).toHaveBeenCalledWith(snapshot.ref, undefined)
  })
})
