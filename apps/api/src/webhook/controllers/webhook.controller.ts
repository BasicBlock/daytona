/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Controller, Get, HttpCode, HttpStatus, NotFoundException, Post } from '@nestjs/common'
import { ApiOperation, ApiResponse, ApiTags } from '@nestjs/swagger'
import { WebhookAppPortalAccessDto } from '../dto/webhook-app-portal-access.dto'
import { WebhookInitializationStatusDto } from '../dto/webhook-initialization-status.dto'
import { WebhookService } from '../services/webhook.service'

@Controller('webhooks')
@ApiTags('webhooks')
export class WebhookController {
  constructor(private readonly webhookService: WebhookService) {}

  @Post('app-portal-access')
  @HttpCode(HttpStatus.OK)
  @ApiOperation({ summary: 'Get Svix Consumer App Portal access' })
  @ApiResponse({
    status: HttpStatus.OK,
    description: 'App Portal access generated successfully',
    type: WebhookAppPortalAccessDto,
  })
  async getAppPortalAccess(): Promise<WebhookAppPortalAccessDto> {
    return this.webhookService.getAppPortalAccess()
  }

  @Get('initialization-status')
  @ApiOperation({ summary: 'Get webhook initialization status' })
  @ApiResponse({
    status: HttpStatus.OK,
    description: 'Webhook initialization status',
    type: WebhookInitializationStatusDto,
  })
  @ApiResponse({
    status: HttpStatus.NOT_FOUND,
    description: 'Webhook initialization status not found',
  })
  async getInitializationStatus(): Promise<WebhookInitializationStatusDto> {
    const status = await this.webhookService.getInitializationStatus()
    if (!status) {
      throw new NotFoundException('Webhook initialization status not found')
    }
    return WebhookInitializationStatusDto.fromWebhookInitialization(status)
  }

  @Post('initialize')
  @HttpCode(HttpStatus.CREATED)
  @ApiOperation({ summary: 'Initialize webhooks' })
  @ApiResponse({
    status: HttpStatus.CREATED,
    description: 'Webhooks initialized successfully',
    type: WebhookInitializationStatusDto,
  })
  async initializeWebhooks(): Promise<WebhookInitializationStatusDto> {
    await this.webhookService.createSvixApplication()
    const status = await this.webhookService.getInitializationStatus()
    if (!status) {
      throw new NotFoundException('Webhook initialization status not found')
    }
    return WebhookInitializationStatusDto.fromWebhookInitialization(status)
  }

  @Post('refresh-endpoints')
  @HttpCode(HttpStatus.NO_CONTENT)
  @ApiOperation({ summary: 'Refresh cached endpoint presence flag' })
  @ApiResponse({
    status: HttpStatus.NO_CONTENT,
    description: 'Endpoint flag refreshed',
  })
  @ApiResponse({
    status: HttpStatus.NOT_FOUND,
    description: 'Webhook initialization status not found',
  })
  async refreshEndpoints(): Promise<void> {
    await this.webhookService.refreshEndpointFlag()
  }
}
