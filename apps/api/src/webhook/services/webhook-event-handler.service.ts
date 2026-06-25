/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Injectable, Logger } from '@nestjs/common'
import { OnEvent } from '@nestjs/event-emitter'
import { SandboxEvents } from '../../sandbox/constants/sandbox-events.constants'
import { SnapshotEvents } from '../../sandbox/constants/snapshot-events'
import { VolumeEvents } from '../../sandbox/constants/volume-events'
import { SandboxCreatedEvent } from '../../sandbox/events/sandbox-create.event'
import { SandboxStateUpdatedEvent } from '../../sandbox/events/sandbox-state-updated.event'
import { SnapshotCreatedEvent } from '../../sandbox/events/snapshot-created.event'
import { SnapshotRemovedEvent } from '../../sandbox/events/snapshot-removed.event'
import { SnapshotStateUpdatedEvent } from '../../sandbox/events/snapshot-state-updated.event'
import { VolumeCreatedEvent } from '../../sandbox/events/volume-created.event'
import { VolumeStateUpdatedEvent } from '../../sandbox/events/volume-state-updated.event'
import { WebhookEvent } from '../constants/webhook-events.constants'
import {
  SandboxCreatedWebhookDto,
  SandboxStateUpdatedWebhookDto,
  SnapshotCreatedWebhookDto,
  SnapshotRemovedWebhookDto,
  SnapshotStateUpdatedWebhookDto,
  VolumeCreatedWebhookDto,
  VolumeStateUpdatedWebhookDto,
} from '../dto/webhook-event-payloads.dto'
import { WebhookService } from './webhook.service'

@Injectable()
export class WebhookEventHandlerService {
  private readonly logger = new Logger(WebhookEventHandlerService.name)

  constructor(private readonly webhookService: WebhookService) {}

  @OnEvent(SandboxEvents.CREATED)
  async handleSandboxCreated(event: SandboxCreatedEvent) {
    await this.sendWebhook(
      WebhookEvent.SANDBOX_CREATED,
      SandboxCreatedWebhookDto.fromEvent(event, WebhookEvent.SANDBOX_CREATED),
      'sandbox created',
    )
  }

  @OnEvent(SandboxEvents.STATE_UPDATED)
  async handleSandboxStateUpdated(event: SandboxStateUpdatedEvent) {
    await this.sendWebhook(
      WebhookEvent.SANDBOX_STATE_UPDATED,
      SandboxStateUpdatedWebhookDto.fromEvent(event, WebhookEvent.SANDBOX_STATE_UPDATED),
      'sandbox state updated',
    )
  }

  @OnEvent(SnapshotEvents.CREATED)
  async handleSnapshotCreated(event: SnapshotCreatedEvent) {
    await this.sendWebhook(
      WebhookEvent.SNAPSHOT_CREATED,
      SnapshotCreatedWebhookDto.fromEvent(event, WebhookEvent.SNAPSHOT_CREATED),
      'snapshot created',
    )
  }

  @OnEvent(SnapshotEvents.STATE_UPDATED)
  async handleSnapshotStateUpdated(event: SnapshotStateUpdatedEvent) {
    await this.sendWebhook(
      WebhookEvent.SNAPSHOT_STATE_UPDATED,
      SnapshotStateUpdatedWebhookDto.fromEvent(event, WebhookEvent.SNAPSHOT_STATE_UPDATED),
      'snapshot state updated',
    )
  }

  @OnEvent(SnapshotEvents.REMOVED)
  async handleSnapshotRemoved(event: SnapshotRemovedEvent) {
    await this.sendWebhook(
      WebhookEvent.SNAPSHOT_REMOVED,
      SnapshotRemovedWebhookDto.fromEvent(event, WebhookEvent.SNAPSHOT_REMOVED),
      'snapshot removed',
    )
  }

  @OnEvent(VolumeEvents.CREATED)
  async handleVolumeCreated(event: VolumeCreatedEvent) {
    await this.sendWebhook(
      WebhookEvent.VOLUME_CREATED,
      VolumeCreatedWebhookDto.fromEvent(event, WebhookEvent.VOLUME_CREATED),
      'volume created',
    )
  }

  @OnEvent(VolumeEvents.STATE_UPDATED)
  async handleVolumeStateUpdated(event: VolumeStateUpdatedEvent) {
    await this.sendWebhook(
      WebhookEvent.VOLUME_STATE_UPDATED,
      VolumeStateUpdatedWebhookDto.fromEvent(event, WebhookEvent.VOLUME_STATE_UPDATED),
      'volume state updated',
    )
  }

  async sendCustomWebhook(eventType: string, payload: unknown, eventId?: string): Promise<void> {
    await this.sendWebhook(eventType, payload, 'custom webhook', eventId)
  }

  private async sendWebhook(eventType: string, payload: unknown, description: string, eventId?: string): Promise<void> {
    if (!this.webhookService.isEnabled()) {
      return
    }

    try {
      await this.webhookService.sendWebhook(eventType, payload, eventId)
    } catch (error) {
      this.logger.error(`Failed to send webhook for ${description}: ${error instanceof Error ? error.message : error}`)
    }
  }
}
