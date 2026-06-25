/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import Redis from 'ioredis'
import { createHash } from 'crypto'
import { Controller, Get, Param, Logger, NotFoundException, UnauthorizedException } from '@nestjs/common'
import { SandboxService } from '../services/sandbox.service'
import { ApiResponse, ApiOperation, ApiParam, ApiTags } from '@nestjs/swagger'
import { InjectRedis } from '@nestjs-modules/ioredis'

@Controller('preview')
@ApiTags('preview')
export class PreviewController {
  private readonly logger = new Logger(PreviewController.name)

  constructor(
    @InjectRedis() private readonly redis: Redis,
    private readonly sandboxService: SandboxService,
  ) {}

  @Get(':sandboxId/open')
  @ApiOperation({
    summary: 'Check if sandbox preview is open',
    operationId: 'isSandboxPreviewOpen',
  })
  @ApiParam({
    name: 'sandboxId',
    description: 'ID of the sandbox',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Open preview status of the sandbox',
    type: Boolean,
  })
  async isSandboxPreviewOpen(@Param('sandboxId') sandboxId: string): Promise<boolean> {
    const cached = await this.redis.get(`preview:open:${sandboxId}`)
    if (cached) {
      if (cached === '1') {
        return true
      }
      throw new NotFoundException(`Sandbox with ID ${sandboxId} not found`)
    }

    try {
      await this.sandboxService.findOne(sandboxId)
      await this.redis.setex(`preview:open:${sandboxId}`, 3, '1')
      return true
    } catch (ex) {
      if (ex instanceof NotFoundException) {
        await this.redis.setex(`preview:open:${sandboxId}`, 3, '0')
        throw ex
      }
      throw ex
    }
  }

  @Get(':sandboxId/validate/:authToken')
  @ApiOperation({
    summary: 'Check if sandbox auth token is valid',
    operationId: 'isValidAuthToken',
  })
  @ApiParam({
    name: 'sandboxId',
    description: 'ID of the sandbox',
    type: 'string',
  })
  @ApiParam({
    name: 'authToken',
    description: 'Auth token of the sandbox',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Sandbox auth token validation status',
    type: Boolean,
  })
  async isValidAuthToken(
    @Param('sandboxId') sandboxId: string,
    @Param('authToken') authToken: string,
  ): Promise<boolean> {
    const tokenHash = createHash('sha256').update(authToken).digest('hex')
    const cacheKey = `preview:token:${sandboxId}:${tokenHash}`
    const cached = await this.redis.get(cacheKey)
    if (cached) {
      if (cached === '1') {
        return true
      }
      throw new UnauthorizedException('Invalid sandbox auth token')
    }

    const sandbox = await this.sandboxService.findOne(sandboxId)
    if (sandbox.authToken !== authToken) {
      await this.redis.setex(cacheKey, 3, '0')
      throw new UnauthorizedException('Invalid sandbox auth token')
    }
    await this.redis.setex(cacheKey, 3, '1')
    return true
  }

  @Get(':sandboxId/access')
  @ApiOperation({
    summary: 'Check if user has access to the sandbox',
    operationId: 'hasSandboxAccess',
  })
  @ApiResponse({
    status: 200,
    description: 'User access status to the sandbox',
    type: Boolean,
  })
  async hasSandboxAccess(@Param('sandboxId') sandboxId: string): Promise<boolean> {
    const cached = await this.redis.get(`preview:access:${sandboxId}`)
    if (cached) {
      if (cached === '1') {
        return true
      }
      throw new NotFoundException(`Sandbox with ID ${sandboxId} not found`)
    }

    await this.sandboxService.findOne(sandboxId)
    await this.redis.setex(`preview:access:${sandboxId}`, 30, '1')
    return true
  }

  @Get(':signedPreviewToken/:port/sandbox-id')
  @ApiOperation({
    summary: 'Get sandbox ID from signed preview URL token',
    operationId: 'getSandboxIdFromSignedPreviewUrlToken',
  })
  @ApiParam({
    name: 'signedPreviewToken',
    description: 'Signed preview URL token',
    type: 'string',
  })
  @ApiParam({
    name: 'port',
    description: 'Port number to get sandbox ID from signed preview URL token',
    type: 'number',
  })
  @ApiResponse({
    status: 200,
    description: 'Sandbox ID from signed preview URL token',
    type: String,
  })
  async getSandboxIdFromSignedPreviewUrlToken(
    @Param('signedPreviewToken') signedPreviewToken: string,
    @Param('port') port: number,
  ): Promise<string> {
    return this.sandboxService.getSandboxIdFromSignedPreviewUrlToken(signedPreviewToken, port)
  }
}
