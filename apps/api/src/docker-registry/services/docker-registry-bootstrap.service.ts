/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Injectable, Logger, OnApplicationBootstrap } from '@nestjs/common'
import { TypedConfigService } from '../../config/typed-config.service'
import { RegistryType } from '../enums/registry-type.enum'
import { DockerRegistryService } from './docker-registry.service'

@Injectable()
export class DockerRegistryBootstrapService implements OnApplicationBootstrap {
  private readonly logger = new Logger(DockerRegistryBootstrapService.name)

  constructor(
    private readonly dockerRegistryService: DockerRegistryService,
    private readonly configService: TypedConfigService,
  ) {}

  async onApplicationBootstrap(): Promise<void> {
    if (this.configService.get('skipConnections')) {
      return
    }

    const target = this.configService.getOrThrow('defaultTarget.id')

    await this.initializeRegistry({
      existing: () => this.dockerRegistryService.getAvailableTransientRegistry(target),
      name: 'Transient Registry',
      registryType: RegistryType.TRANSIENT,
      configPrefix: 'transientRegistry',
    })
    await this.initializeRegistry({
      existing: () => this.dockerRegistryService.getAvailableInternalRegistry(target),
      name: 'Internal Registry',
      registryType: RegistryType.INTERNAL,
      configPrefix: 'internalRegistry',
    })
    await this.initializeRegistry({
      existing: () => this.dockerRegistryService.getAvailableBackupRegistry(target),
      name: 'Backup Registry',
      registryType: RegistryType.BACKUP,
      configPrefix: 'internalRegistry',
      isFallback: true,
    })
  }

  private async initializeRegistry(options: {
    existing: () => Promise<unknown>
    name: string
    registryType: RegistryType
    configPrefix: 'internalRegistry' | 'transientRegistry'
    isFallback?: boolean
  }): Promise<void> {
    if (await options.existing()) {
      return
    }

    const registryUrl = this.configService.get(`${options.configPrefix}.url`)
    const registryAdmin = this.configService.get(`${options.configPrefix}.admin`)
    const registryPassword = this.configService.get(`${options.configPrefix}.password`)
    const registryProjectId = this.configService.get(`${options.configPrefix}.projectId`)

    if (!registryUrl || !registryAdmin || !registryPassword || !registryProjectId) {
      this.logger.warn(`${options.name} configuration not found, skipping registry setup`)
      return
    }

    await this.dockerRegistryService.create(
      {
        name: options.name,
        url: registryUrl,
        username: registryAdmin,
        password: registryPassword,
        project: registryProjectId,
        registryType: options.registryType,
        isDefault: true,
      },
      options.isFallback,
    )

    this.logger.log(`${options.name} initialized successfully`)
  }
}
