/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Module } from '@nestjs/common'
import { DataSource } from 'typeorm'
import { SandboxController } from './controllers/sandbox.controller'
import { SandboxService } from './services/sandbox.service'
import { TypeOrmModule } from '@nestjs/typeorm'
import { Sandbox } from './entities/sandbox.entity'
import { RunnerService } from './services/runner.service'
import { Runner } from './entities/runner.entity'
import { RunnerController } from './controllers/runner.controller'
import { DockerRegistryModule } from '../docker-registry/docker-registry.module'
import { Snapshot } from './entities/snapshot.entity'
import { SnapshotController } from './controllers/snapshot.controller'
import { SnapshotService } from './services/snapshot.service'
import { SnapshotRunner } from './entities/snapshot-runner.entity'
import { DockerRegistry } from '../docker-registry/entities/docker-registry.entity'
import { RedisLockProvider } from './common/redis-lock.provider'
import { SandboxWarmPoolService } from './services/sandbox-warm-pool.service'
import { WarmPool } from './entities/warm-pool.entity'
import { PreviewController } from './controllers/preview.controller'
import { SnapshotRepository } from './repositories/snapshot.repository'
import { VolumeController } from './controllers/volume.controller'
import { VolumeService } from './services/volume.service'
import { VolumeManager } from './managers/volume.manager'
import { Volume } from './entities/volume.entity'
import { BuildInfo } from './entities/build-info.entity'
import { BackupManager } from './managers/backup.manager'
import { VolumeSubscriber } from './subscribers/volume.subscriber'
import { RunnerSubscriber } from './subscribers/runner.subscriber'
import { RunnerAdapterFactory } from './runner-adapter/runnerAdapter'
import { SandboxStartAction } from './managers/sandbox-actions/sandbox-start.action'
import { SandboxStopAction } from './managers/sandbox-actions/sandbox-stop.action'
import { SandboxDestroyAction } from './managers/sandbox-actions/sandbox-destroy.action'
import { SandboxArchiveAction } from './managers/sandbox-actions/sandbox-archive.action'
import { SandboxManager } from './managers/sandbox.manager'
import { SshAccess } from './entities/ssh-access.entity'
import { SandboxRepository } from './repositories/sandbox.repository'
import { ProxyCacheInvalidationService } from './services/proxy-cache-invalidation.service'
import { SandboxFork } from './entities/sandbox-fork.entity'
import { JobController } from './controllers/job.controller'
import { JobService } from './services/job.service'
import { JobStateHandlerService } from './services/job-state-handler.service'
import { Job } from './entities/job.entity'
import { SandboxLookupCacheInvalidationService } from './services/sandbox-lookup-cache-invalidation.service'
import { EventEmitter2 } from '@nestjs/event-emitter'
import { SandboxLastActivity } from './entities/sandbox-last-activity.entity'
import { SandboxActivityService } from './services/sandbox-activity.service'
import { OpensearchModule } from 'nestjs-opensearch'
import { TypedConfigService } from '../config/typed-config.service'
import { SandboxSearchAdapterProvider } from './providers/sandbox-search.provider'

@Module({
  imports: [
    DockerRegistryModule,
    TypeOrmModule.forFeature([
      Sandbox,
      Runner,
      Snapshot,
      BuildInfo,
      SnapshotRunner,
      DockerRegistry,
      WarmPool,
      Volume,
      SshAccess,
      Job,
      SandboxLastActivity,
      SandboxFork,
    ]),
    OpensearchModule.forRootAsync({
      inject: [TypedConfigService],
      useFactory: (configService: TypedConfigService) => {
        return configService.getOpenSearchConfig()
      },
    }),
  ],
  controllers: [
    SandboxController,
    RunnerController,
    SnapshotController,
    PreviewController,
    VolumeController,
    JobController,
  ],
  providers: [
    SandboxService,
    SandboxManager,
    BackupManager,
    SandboxWarmPoolService,
    RunnerService,
    SnapshotService,
    ProxyCacheInvalidationService,
    SandboxLookupCacheInvalidationService,
    RedisLockProvider,
    VolumeService,
    VolumeManager,
    VolumeSubscriber,
    RunnerSubscriber,
    RunnerAdapterFactory,
    SandboxStartAction,
    SandboxStopAction,
    SandboxDestroyAction,
    SandboxArchiveAction,
    JobService,
    JobStateHandlerService,
    SandboxActivityService,
    SandboxSearchAdapterProvider,
    {
      provide: SandboxRepository,
      inject: [DataSource, EventEmitter2, SandboxLookupCacheInvalidationService],
      useFactory: (
        dataSource: DataSource,
        eventEmitter: EventEmitter2,
        sandboxLookupCacheInvalidationService: SandboxLookupCacheInvalidationService,
      ) => new SandboxRepository(dataSource, eventEmitter, sandboxLookupCacheInvalidationService),
    },
    {
      provide: SnapshotRepository,
      inject: [DataSource, EventEmitter2],
      useFactory: (dataSource: DataSource, eventEmitter: EventEmitter2) =>
        new SnapshotRepository(dataSource, eventEmitter),
    },
  ],
  exports: [
    SandboxService,
    RunnerService,
    RedisLockProvider,
    SnapshotService,
    VolumeService,
    VolumeManager,
    SandboxRepository,
    SnapshotRepository,
    RunnerAdapterFactory,
    SandboxActivityService,
  ],
})
export class SandboxModule {}
