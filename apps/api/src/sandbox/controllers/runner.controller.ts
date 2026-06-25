/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  NotFoundException,
  Param,
  ParseUUIDPipe,
  Patch,
  Post,
  Query,
} from '@nestjs/common'
import { ApiOperation, ApiParam, ApiQuery, ApiResponse, ApiTags } from '@nestjs/swagger'
import { BadRequestError } from '../../exceptions/bad-request.exception'
import { CreateRunnerDto } from '../dto/create-runner.dto'
import { CreateRunnerResponseDto } from '../dto/create-runner-response.dto'
import { RunnerFullDto } from '../dto/runner-full.dto'
import { RunnerHealthcheckDto } from '../dto/runner-health.dto'
import { RunnerDto } from '../dto/runner.dto'
import { RunnerSnapshotDto } from '../dto/runner-snapshot.dto'
import { RunnerService } from '../services/runner.service'

@Controller('runners')
@ApiTags('runners')
export class RunnerController {
  constructor(private readonly runnerService: RunnerService) {}

  @Post()
  @HttpCode(201)
  @ApiOperation({ summary: 'Create runner', operationId: 'createRunner' })
  @ApiResponse({ status: 201, type: CreateRunnerResponseDto })
  async create(@Body() createRunnerDto: CreateRunnerDto): Promise<CreateRunnerResponseDto> {
    const { runner } = await this.runnerService.create({
      target: createRunnerDto.target,
      name: createRunnerDto.name,
      apiVersion: '2',
      tags: createRunnerDto.tags,
    })

    return CreateRunnerResponseDto.fromRunner(runner)
  }

  @Get('/me')
  @ApiOperation({ summary: 'Get runner info', operationId: 'getInfoForRunner' })
  @ApiQuery({ name: 'runnerId', required: true, type: String })
  @ApiResponse({ status: 200, type: RunnerFullDto })
  async getInfoForRunner(@Query('runnerId') runnerId: string): Promise<RunnerFullDto> {
    if (!runnerId) {
      throw new BadRequestError('runnerId is required')
    }
    return this.runnerService.findOneFullOrFail(runnerId)
  }

  @Get('/by-sandbox/:sandboxId')
  @HttpCode(200)
  @ApiOperation({ summary: 'Get runner by sandbox ID', operationId: 'getRunnerBySandboxId' })
  @ApiParam({ name: 'sandboxId', type: String })
  @ApiResponse({ status: 200, type: RunnerFullDto })
  async getRunnerBySandboxId(@Param('sandboxId') sandboxId: string): Promise<RunnerFullDto> {
    const runner = await this.runnerService.findBySandboxId(sandboxId)
    if (!runner) {
      throw new NotFoundException('Runner not found')
    }
    return RunnerFullDto.fromRunner(runner)
  }

  @Get('/by-snapshot-ref')
  @HttpCode(200)
  @ApiOperation({ summary: 'Get runners by snapshot ref', operationId: 'getRunnersBySnapshotRef' })
  @ApiQuery({ name: 'ref', type: String, required: true })
  @ApiResponse({ status: 200, type: [RunnerSnapshotDto] })
  async getRunnersBySnapshotRef(@Query('ref') ref: string): Promise<RunnerSnapshotDto[]> {
    return this.runnerService.getRunnersBySnapshotRef(ref)
  }

  @Get(':id')
  @HttpCode(200)
  @ApiOperation({ summary: 'Get runner by ID', operationId: 'getRunnerById' })
  @ApiParam({ name: 'id', type: String })
  @ApiResponse({ status: 200, type: RunnerDto })
  async getRunnerById(@Param('id', ParseUUIDPipe) id: string): Promise<RunnerDto> {
    return RunnerDto.fromRunner(await this.runnerService.findOneOrFail(id))
  }

  @Get(':id/full')
  @HttpCode(200)
  @ApiOperation({ summary: 'Get runner by ID', operationId: 'getRunnerFullById' })
  @ApiParam({ name: 'id', type: String })
  @ApiResponse({ status: 200, type: RunnerFullDto })
  async getRunnerByIdFull(@Param('id', ParseUUIDPipe) id: string): Promise<RunnerFullDto> {
    return RunnerFullDto.fromRunner(await this.runnerService.findOneOrFail(id))
  }

  @Get()
  @HttpCode(200)
  @ApiOperation({ summary: 'List all runners', operationId: 'listRunners' })
  @ApiQuery({ name: 'target', type: String, required: false })
  @ApiResponse({ status: 200, type: [RunnerDto] })
  async findAll(@Query('target') target?: string): Promise<RunnerDto[]> {
    if (target) {
      return this.runnerService.findAllByTarget(target)
    }
    return this.runnerService.findAll()
  }

  @Patch(':id/scheduling')
  @HttpCode(200)
  @ApiOperation({ summary: 'Update runner scheduling status', operationId: 'updateRunnerScheduling' })
  @ApiParam({ name: 'id', type: String })
  @ApiResponse({ status: 200, type: RunnerDto })
  async updateSchedulingStatus(
    @Param('id', ParseUUIDPipe) id: string,
    @Body('unschedulable') unschedulable: boolean,
  ): Promise<RunnerDto> {
    return RunnerDto.fromRunner(await this.runnerService.updateSchedulingStatus(id, unschedulable))
  }

  @Patch(':id/draining')
  @HttpCode(200)
  @ApiOperation({ summary: 'Update runner draining status', operationId: 'updateRunnerDraining' })
  @ApiParam({ name: 'id', type: String })
  @ApiResponse({ status: 200, type: RunnerDto })
  async updateDrainingStatus(
    @Param('id', ParseUUIDPipe) id: string,
    @Body('draining') draining: boolean,
  ): Promise<RunnerDto> {
    return RunnerDto.fromRunner(await this.runnerService.updateDrainingStatus(id, draining))
  }

  @Delete(':id')
  @HttpCode(204)
  @ApiOperation({ summary: 'Delete runner', operationId: 'deleteRunner' })
  @ApiParam({ name: 'id', type: String })
  @ApiResponse({ status: 204 })
  async delete(@Param('id', ParseUUIDPipe) id: string): Promise<void> {
    return this.runnerService.remove(id)
  }

  @Post('healthcheck')
  @ApiOperation({ summary: 'Runner healthcheck', operationId: 'runnerHealthcheck' })
  @ApiQuery({ name: 'runnerId', required: true, type: String })
  @ApiResponse({ status: 200 })
  async runnerHealthcheck(
    @Query('runnerId') runnerId: string,
    @Body() healthcheck: RunnerHealthcheckDto,
  ): Promise<void> {
    if (!runnerId) {
      throw new BadRequestError('runnerId is required')
    }
    await this.runnerService.updateRunnerHealth(
      runnerId,
      healthcheck.domain,
      healthcheck.apiUrl,
      healthcheck.proxyUrl,
      healthcheck.serviceHealth,
      healthcheck.metrics,
      healthcheck.appVersion,
    )
  }
}
