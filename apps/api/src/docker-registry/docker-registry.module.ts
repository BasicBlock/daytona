/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Module } from '@nestjs/common'
import { TypeOrmModule } from '@nestjs/typeorm'
import { DockerRegistry } from './entities/docker-registry.entity'
import { DockerRegistryService } from './services/docker-registry.service'
import { EcrCredentialsService } from './services/ecr-credentials.service'
import { DockerRegistryController } from './controllers/docker-registry.controller'
import { HttpModule } from '@nestjs/axios'
import { DockerRegistryProvider } from './providers/docker-registry.provider'
import { DOCKER_REGISTRY_PROVIDER } from './providers/docker-registry.provider.interface'
import { DockerRegistryBootstrapService } from './services/docker-registry-bootstrap.service'

@Module({
  imports: [TypeOrmModule.forFeature([DockerRegistry]), HttpModule],
  controllers: [DockerRegistryController],
  providers: [
    {
      provide: DOCKER_REGISTRY_PROVIDER,
      useClass: DockerRegistryProvider,
    },
    DockerRegistryService,
    DockerRegistryBootstrapService,
    EcrCredentialsService,
  ],
  exports: [DockerRegistryService],
})
export class DockerRegistryModule {}
