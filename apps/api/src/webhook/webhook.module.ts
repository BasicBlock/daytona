/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Module } from '@nestjs/common'
import { TypeOrmModule } from '@nestjs/typeorm'
import { TypedConfigModule } from '../config/typed-config.module'
import { WebhookController } from './controllers/webhook.controller'
import { WebhookInitialization } from './entities/webhook-initialization.entity'
import { WebhookEventHandlerService } from './services/webhook-event-handler.service'
import { WebhookService } from './services/webhook.service'

@Module({
  imports: [TypedConfigModule, TypeOrmModule.forFeature([WebhookInitialization])],
  controllers: [WebhookController],
  providers: [WebhookService, WebhookEventHandlerService],
  exports: [WebhookService],
})
export class WebhookModule {}
