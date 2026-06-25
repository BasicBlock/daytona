/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ApiProperty, ApiSchema } from '@nestjs/swagger'
import { WebhookInitialization } from '../entities/webhook-initialization.entity'

@ApiSchema({ name: 'WebhookInitializationStatus' })
export class WebhookInitializationStatusDto {
  @ApiProperty({
    description: 'Webhook application ID',
    example: 'default',
  })
  applicationId: string

  @ApiProperty({
    description: 'The ID of the Svix application',
    example: 'app_1234567890',
    nullable: true,
  })
  svixApplicationId?: string

  @ApiProperty({
    description: 'The error reason for the last initialization attempt',
    example: 'Failed to create Svix application',
    nullable: true,
  })
  lastError?: string

  @ApiProperty({
    description: 'The number of times the initialization has been attempted',
    example: 3,
  })
  retryCount: number

  @ApiProperty({
    description: 'When the webhook initialization was created',
    example: '2023-01-01T00:00:00.000Z',
  })
  createdAt: string

  @ApiProperty({
    description: 'When the webhook initialization was last updated',
    example: '2023-01-01T00:00:00.000Z',
  })
  updatedAt: string

  static fromWebhookInitialization(webhookInitialization: WebhookInitialization): WebhookInitializationStatusDto {
    return {
      applicationId: webhookInitialization.applicationId,
      svixApplicationId: webhookInitialization.svixApplicationId,
      lastError: webhookInitialization.lastError,
      retryCount: webhookInitialization.retryCount,
      createdAt: webhookInitialization.createdAt.toISOString(),
      updatedAt: webhookInitialization.updatedAt.toISOString(),
    }
  }
}
