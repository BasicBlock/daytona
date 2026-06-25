/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Controller, Get, Post, Body, Patch, Param, Delete, HttpCode } from '@nestjs/common'
import { ApiTags, ApiOperation, ApiResponse, ApiParam } from '@nestjs/swagger'
import { DockerRegistryService } from '../services/docker-registry.service'
import { CreateDockerRegistryDto } from '../dto/create-docker-registry.dto'
import { UpdateDockerRegistryDto } from '../dto/update-docker-registry.dto'
import { DockerRegistryDto } from '../dto/docker-registry.dto'
import { RegistryPushAccessDto } from '../../sandbox/dto/registry-push-access-dto'
import { RegistryType } from '../enums/registry-type.enum'

@Controller('docker-registry')
@ApiTags('docker-registry')
export class DockerRegistryController {
  constructor(private readonly dockerRegistryService: DockerRegistryService) {}

  @Post()
  @ApiOperation({
    summary: 'Create registry',
    operationId: 'createRegistry',
  })
  @ApiResponse({
    status: 201,
    description: 'The docker registry has been successfully created.',
    type: DockerRegistryDto,
  })
  async create(@Body() createDockerRegistryDto: CreateDockerRegistryDto): Promise<DockerRegistryDto> {
    const dockerRegistry = await this.dockerRegistryService.create({
      ...createDockerRegistryDto,
      registryType: RegistryType.CUSTOM,
    })
    return DockerRegistryDto.fromDockerRegistry(dockerRegistry)
  }

  @Get()
  @ApiOperation({
    summary: 'List registries',
    operationId: 'listRegistries',
  })
  @ApiResponse({
    status: 200,
    description: 'List of all docker registries',
    type: [DockerRegistryDto],
  })
  async findAll(): Promise<DockerRegistryDto[]> {
    const dockerRegistries = await this.dockerRegistryService.findAll(RegistryType.CUSTOM)
    return dockerRegistries.map(DockerRegistryDto.fromDockerRegistry)
  }

  @Get('registry-push-access')
  @HttpCode(200)
  @ApiOperation({
    summary: 'Get temporary registry access for pushing snapshots',
    operationId: 'getTransientPushAccess',
  })
  @ApiResponse({
    status: 200,
    description: 'Temporary registry access has been generated',
    type: RegistryPushAccessDto,
  })
  async getTransientPushAccess(): Promise<RegistryPushAccessDto> {
    return this.dockerRegistryService.getRegistryPushAccess()
  }

  @Get(':id')
  @ApiOperation({
    summary: 'Get registry',
    operationId: 'getRegistry',
  })
  @ApiParam({
    name: 'id',
    description: 'ID of the docker registry',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'The docker registry',
    type: DockerRegistryDto,
  })
  async findOne(@Param('id') registryId: string): Promise<DockerRegistryDto> {
    const registry = await this.dockerRegistryService.findOneOrFail(registryId)
    return DockerRegistryDto.fromDockerRegistry(registry)
  }

  @Patch(':id')
  @ApiOperation({
    summary: 'Update registry',
    operationId: 'updateRegistry',
  })
  @ApiParam({
    name: 'id',
    description: 'ID of the docker registry',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'The docker registry has been successfully updated.',
    type: DockerRegistryDto,
  })
  async update(
    @Param('id') registryId: string,
    @Body() updateDockerRegistryDto: UpdateDockerRegistryDto,
  ): Promise<DockerRegistryDto> {
    const dockerRegistry = await this.dockerRegistryService.update(registryId, updateDockerRegistryDto)
    return DockerRegistryDto.fromDockerRegistry(dockerRegistry)
  }

  @Delete(':id')
  @HttpCode(204)
  @ApiOperation({
    summary: 'Delete registry',
    operationId: 'deleteRegistry',
  })
  @ApiParam({
    name: 'id',
    description: 'ID of the docker registry',
    type: 'string',
  })
  @ApiResponse({
    status: 204,
    description: 'The docker registry has been successfully deleted.',
  })
  async remove(@Param('id') registryId: string): Promise<void> {
    return this.dockerRegistryService.remove(registryId)
  }
}
