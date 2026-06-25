/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import {
  Body,
  Controller,
  Delete,
  Get,
  Param,
  Post,
  Query,
  HttpCode,
  NotFoundException,
  Res,
  Request,
  RawBodyRequest,
  Next,
  ParseBoolPipe,
  ParseUUIDPipe,
} from '@nestjs/common'
import { IncomingMessage, ServerResponse } from 'http'
import { NextFunction } from 'express'
import { ApiTags, ApiOperation, ApiResponse, ApiParam, ApiQuery } from '@nestjs/swagger'
import { SnapshotService } from '../services/snapshot.service'
import { RunnerService } from '../services/runner.service'
import { CreateSnapshotDto } from '../dto/create-snapshot.dto'
import { SnapshotDto } from '../dto/snapshot.dto'
import { PaginatedSnapshotsDto } from '../dto/paginated-snapshots.dto'
import { LogProxy } from '../proxy/log-proxy'
import { BadRequestError } from '../../exceptions/bad-request.exception'
import { ListSnapshotsQueryDto } from '../dto/list-snapshots-query.dto'
import { SnapshotState } from '../enums/snapshot-state.enum'
import { UrlDto } from '../../common/dto/url.dto'

@Controller('snapshots')
@ApiTags('snapshots')
export class SnapshotController {
  constructor(
    private readonly snapshotService: SnapshotService,
    private readonly runnerService: RunnerService,
  ) {}

  @Post()
  @HttpCode(200)
  @ApiOperation({
    summary: 'Create a new snapshot',
    operationId: 'createSnapshot',
  })
  @ApiResponse({
    status: 200,
    description: 'The snapshot has been successfully created.',
    type: SnapshotDto,
  })
  @ApiResponse({
    status: 400,
    description: 'Bad request - Snapshots with tag ":latest" are not allowed',
  })
  async createSnapshot(@Body() createSnapshotDto: CreateSnapshotDto): Promise<SnapshotDto> {
    if (createSnapshotDto.buildInfo) {
      if (createSnapshotDto.imageName) {
        throw new BadRequestError('Cannot specify an image name when using a build info entry')
      }
      if (createSnapshotDto.entrypoint) {
        throw new BadRequestError('Cannot specify an entrypoint when using a build info entry')
      }
    } else if (!createSnapshotDto.imageName) {
      throw new BadRequestError('Must specify an image name when not using a build info entry')
    }

    const snapshot = createSnapshotDto.buildInfo
      ? await this.snapshotService.createFromBuildInfo(createSnapshotDto)
      : await this.snapshotService.createFromPull(createSnapshotDto)
    return SnapshotDto.fromSnapshot(snapshot)
  }

  @Get(':id')
  @ApiOperation({
    summary: 'Get snapshot by ID or name',
    operationId: 'getSnapshot',
  })
  @ApiParam({
    name: 'id',
    description: 'Snapshot ID or name',
  })
  @ApiResponse({
    status: 200,
    description: 'The snapshot',
    type: SnapshotDto,
  })
  @ApiResponse({
    status: 404,
    description: 'Snapshot not found',
  })
  async getSnapshot(@Param('id') snapshotIdOrName: string): Promise<SnapshotDto> {
    const snapshot = await this.snapshotService.getSnapshot(snapshotIdOrName)
    return SnapshotDto.fromSnapshot(snapshot)
  }

  @Delete(':id')
  @ApiOperation({
    summary: 'Delete snapshot',
    operationId: 'removeSnapshot',
  })
  @ApiParam({
    name: 'id',
    description: 'Snapshot ID',
  })
  @ApiResponse({
    status: 200,
    description: 'Snapshot has been deleted',
  })
  async removeSnapshot(@Param('id', ParseUUIDPipe) snapshotId: string): Promise<void> {
    await this.snapshotService.removeSnapshot(snapshotId)
  }

  @Get()
  @ApiOperation({
    summary: 'List all snapshots',
    operationId: 'getAllSnapshots',
  })
  @ApiResponse({
    status: 200,
    description: 'Paginated list of all snapshots',
    type: PaginatedSnapshotsDto,
  })
  async getAllSnapshots(@Query() queryParams: ListSnapshotsQueryDto): Promise<PaginatedSnapshotsDto> {
    const { page, limit, name, sort, order } = queryParams

    const result = await this.snapshotService.getAllSnapshots(page, limit, { name }, { field: sort, direction: order })

    return {
      items: result.items.map(SnapshotDto.fromSnapshot),
      total: result.total,
      page: result.page,
      totalPages: result.totalPages,
    }
  }

  @Get(':id/build-logs')
  @ApiOperation({
    summary: 'Get snapshot build logs',
    operationId: 'getSnapshotBuildLogs',
    deprecated: true,
    description: 'This endpoint is deprecated. Use `getSnapshotBuildLogsUrl` instead.',
  })
  @ApiParam({
    name: 'id',
    description: 'Snapshot ID',
  })
  @ApiQuery({
    name: 'follow',
    required: false,
    type: Boolean,
    description: 'Whether to follow the logs stream',
  })
  async getSnapshotBuildLogs(
    @Request() req: RawBodyRequest<IncomingMessage>,
    @Res() res: ServerResponse<IncomingMessage>,
    @Next() next: NextFunction,
    @Param('id') snapshotId: string,
    @Query('follow', new ParseBoolPipe({ optional: true })) follow?: boolean,
  ): Promise<void> {
    let snapshot = await this.snapshotService.getSnapshot(snapshotId)

    if (!snapshot.buildInfo) {
      throw new NotFoundException(`Snapshot ${snapshotId} has no build info`)
    }

    if (snapshot.state === SnapshotState.ACTIVE) {
      res.end()
      return
    }

    const startTime = Date.now()
    const timeoutMs = 30 * 1000

    while (!snapshot.initialRunnerId) {
      if (Date.now() - startTime > timeoutMs) {
        throw new NotFoundException(`Timeout waiting for build runner assignment for snapshot ${snapshotId}`)
      }
      await new Promise((resolve) => setTimeout(resolve, 1000))
      snapshot = await this.snapshotService.getSnapshot(snapshotId)
    }

    const runner = await this.runnerService.findOneOrFail(snapshot.initialRunnerId)
    if (!runner.apiUrl) {
      throw new NotFoundException(`Build runner for snapshot ${snapshotId} has no API URL`)
    }

    const logProxy = new LogProxy(runner.apiUrl, snapshot.buildInfo.snapshotRef, follow === true, req, res, next)
    return logProxy.create()
  }

  @Get(':id/build-logs-url')
  @ApiOperation({
    summary: 'Get snapshot build logs URL',
    operationId: 'getSnapshotBuildLogsUrl',
  })
  @ApiParam({
    name: 'id',
    description: 'Snapshot ID',
  })
  @ApiResponse({
    status: 200,
    description: 'The snapshot build logs URL',
    type: UrlDto,
  })
  async getSnapshotBuildLogsUrl(@Param('id') snapshotId: string): Promise<UrlDto> {
    let snapshot = await this.snapshotService.getSnapshot(snapshotId)
    if (!snapshot.buildInfo) {
      throw new NotFoundException(`Snapshot ${snapshotId} has no build info`)
    }

    const startTime = Date.now()
    const timeoutMs = 30 * 1000

    while (!snapshot.initialRunnerId) {
      if (Date.now() - startTime > timeoutMs) {
        throw new NotFoundException(`Timeout waiting for build runner assignment for snapshot ${snapshotId}`)
      }
      await new Promise((resolve) => setTimeout(resolve, 1000))
      snapshot = await this.snapshotService.getSnapshot(snapshotId)
    }

    return new UrlDto(await this.snapshotService.getBuildLogsUrl(snapshot))
  }

  @Post(':id/activate')
  @HttpCode(200)
  @ApiOperation({
    summary: 'Activate a snapshot',
    operationId: 'activateSnapshot',
  })
  @ApiParam({
    name: 'id',
    description: 'Snapshot ID',
  })
  @ApiResponse({
    status: 200,
    description: 'The snapshot has been successfully activated.',
    type: SnapshotDto,
  })
  async activateSnapshot(@Param('id', ParseUUIDPipe) snapshotId: string): Promise<SnapshotDto> {
    const snapshot = await this.snapshotService.activateSnapshot(snapshotId)
    return SnapshotDto.fromSnapshot(snapshot)
  }

  @Post(':id/deactivate')
  @HttpCode(204)
  @ApiOperation({
    summary: 'Deactivate a snapshot',
    operationId: 'deactivateSnapshot',
  })
  @ApiParam({
    name: 'id',
    description: 'Snapshot ID',
  })
  @ApiResponse({
    status: 204,
    description: 'The snapshot has been successfully deactivated.',
  })
  async deactivateSnapshot(@Param('id', ParseUUIDPipe) snapshotId: string): Promise<void> {
    await this.snapshotService.deactivateSnapshot(snapshotId)
  }
}
