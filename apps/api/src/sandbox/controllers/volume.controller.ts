/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Controller, Get, Post, Delete, Body, Param, Logger, HttpCode, Query } from '@nestjs/common'
import { ApiResponse, ApiOperation, ApiParam, ApiTags, ApiQuery } from '@nestjs/swagger'
import { VolumeService } from '../services/volume.service'
import { CreateVolumeDto } from '../dto/create-volume.dto'
import { VolumeDto } from '../dto/volume.dto'

@Controller('volumes')
@ApiTags('volumes')
export class VolumeController {
  private readonly logger = new Logger(VolumeController.name)

  constructor(private readonly volumeService: VolumeService) {}

  @Get()
  @ApiOperation({
    summary: 'List all volumes',
    operationId: 'listVolumes',
  })
  @ApiQuery({
    name: 'includeDeleted',
    required: false,
    type: Boolean,
    description: 'Include deleted volumes in the response',
  })
  @ApiResponse({
    status: 200,
    description: 'List of all volumes',
    type: [VolumeDto],
  })
  async listVolumes(@Query('includeDeleted') includeDeleted = false): Promise<VolumeDto[]> {
    const volumes = await this.volumeService.findAll(includeDeleted)
    return volumes.map(VolumeDto.fromVolume)
  }

  @Post()
  @HttpCode(200)
  @ApiOperation({
    summary: 'Create a new volume',
    operationId: 'createVolume',
  })
  @ApiResponse({
    status: 200,
    description: 'The volume has been successfully created.',
    type: VolumeDto,
  })
  async createVolume(@Body() createVolumeDto: CreateVolumeDto): Promise<VolumeDto> {
    const volume = await this.volumeService.create(createVolumeDto)
    return VolumeDto.fromVolume(volume)
  }

  @Get(':volumeId')
  @ApiOperation({
    summary: 'Get volume details',
    operationId: 'getVolume',
  })
  @ApiParam({
    name: 'volumeId',
    description: 'ID of the volume',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Volume details',
    type: VolumeDto,
  })
  async getVolume(@Param('volumeId') volumeId: string): Promise<VolumeDto> {
    const volume = await this.volumeService.findOne(volumeId)
    return VolumeDto.fromVolume(volume)
  }

  @Delete(':volumeId')
  @ApiOperation({
    summary: 'Delete volume',
    operationId: 'deleteVolume',
  })
  @ApiParam({
    name: 'volumeId',
    description: 'ID of the volume',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Volume has been marked for deletion',
  })
  @ApiResponse({
    status: 409,
    description: 'Volume is in use by one or more sandboxes',
  })
  async deleteVolume(@Param('volumeId') volumeId: string): Promise<void> {
    return this.volumeService.delete(volumeId)
  }

  @Get('by-name/:name')
  @ApiOperation({
    summary: 'Get volume details by name',
    operationId: 'getVolumeByName',
  })
  @ApiParam({
    name: 'name',
    description: 'Name of the volume',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Volume details',
    type: VolumeDto,
  })
  async getVolumeByName(@Param('name') name: string): Promise<VolumeDto> {
    const volume = await this.volumeService.findByName(name)
    return VolumeDto.fromVolume(volume)
  }
}
