/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Injectable, Logger, NotFoundException, OnModuleInit, ServiceUnavailableException } from '@nestjs/common'
import { InjectRepository } from '@nestjs/typeorm'
import { Repository } from 'typeorm'
import { Svix } from 'svix'
import { TypedConfigService } from '../../config/typed-config.service'
import { WebhookInitialization } from '../entities/webhook-initialization.entity'

export const DEFAULT_WEBHOOK_APPLICATION_ID = 'default'
const DEFAULT_WEBHOOK_APPLICATION_NAME = 'Daytona'

@Injectable()
export class WebhookService implements OnModuleInit {
  private static readonly ENDPOINT_FLAG_TTL_MS = 60_000

  private readonly logger = new Logger(WebhookService.name)
  private svix: Svix | null = null

  constructor(
    private readonly configService: TypedConfigService,
    @InjectRepository(WebhookInitialization)
    private readonly webhookInitializationRepository: Repository<WebhookInitialization>,
  ) {}

  async onModuleInit() {
    const svixAuthToken = this.configService.get('webhook.authToken')
    if (svixAuthToken) {
      const serverUrl = this.configService.get('webhook.serverUrl')
      this.svix = serverUrl ? new Svix(svixAuthToken, { serverUrl }) : new Svix(svixAuthToken)
      this.logger.log('Svix webhook service initialized')
    } else {
      this.logger.warn('SVIX_AUTH_TOKEN not configured, webhook service disabled')
    }
  }

  async getInitializationStatus(applicationId = DEFAULT_WEBHOOK_APPLICATION_ID): Promise<WebhookInitialization | null> {
    return this.webhookInitializationRepository.findOne({
      where: { applicationId },
    })
  }

  async createSvixApplication(
    applicationId = DEFAULT_WEBHOOK_APPLICATION_ID,
    applicationName = DEFAULT_WEBHOOK_APPLICATION_NAME,
  ): Promise<string> {
    if (!this.svix) {
      throw new ServiceUnavailableException('Webhook service is not configured')
    }

    let existingWebhookInitialization = await this.getInitializationStatus(applicationId)
    if (existingWebhookInitialization?.svixApplicationId) {
      this.logger.warn(
        `Svix application already exists for ${applicationId}: ${existingWebhookInitialization.svixApplicationId}`,
      )
      return existingWebhookInitialization.svixApplicationId
    }

    if (!existingWebhookInitialization) {
      existingWebhookInitialization = new WebhookInitialization()
      existingWebhookInitialization.applicationId = applicationId
      existingWebhookInitialization.svixApplicationId = null
      existingWebhookInitialization.retryCount = -1
      existingWebhookInitialization.lastError = null
    }

    try {
      const svixApp = await this.svix.application.getOrCreate({
        name: applicationName,
        uid: applicationId,
      })
      existingWebhookInitialization.svixApplicationId = svixApp.id
      existingWebhookInitialization.retryCount += 1
      existingWebhookInitialization.lastError = null

      await this.webhookInitializationRepository.save(existingWebhookInitialization)

      this.logger.log(`Created Svix application for ${applicationId}: ${svixApp.id}`)
      return svixApp.id
    } catch (error) {
      existingWebhookInitialization.retryCount += 1
      existingWebhookInitialization.lastError = String(error)
      await this.webhookInitializationRepository.save(existingWebhookInitialization)
      this.logger.error(`Failed to create Svix application for ${applicationId}:`, error)
      throw error
    }
  }

  private async refreshEndpointFlagForInitialization(init: WebhookInitialization): Promise<WebhookInitialization> {
    if (!this.svix) {
      return init
    }

    try {
      const result = await this.svix.endpoint.list(init.applicationId, { limit: 1 })
      return await this.webhookInitializationRepository.save({
        ...init,
        hasEndpoints: result.data.length > 0,
        endpointsCheckedAt: new Date(),
      })
    } catch (error) {
      this.logger.error(`Failed to refresh endpoint flag for ${init.applicationId}:`, error)
      return init
    }
  }

  async refreshEndpointFlag(applicationId = DEFAULT_WEBHOOK_APPLICATION_ID): Promise<void> {
    const init = await this.getInitializationStatus(applicationId)
    if (!init) {
      throw new NotFoundException('Webhook initialization status not found')
    }
    await this.refreshEndpointFlagForInitialization(init)
  }

  async sendWebhook(eventType: string, payload: unknown, eventId?: string): Promise<void> {
    if (!this.svix) {
      this.logger.debug('Svix not configured, skipping webhook delivery')
      return
    }

    try {
      let init = await this.getInitializationStatus()
      if (!init) {
        this.logger.debug(`Skipping webhook ${eventType}: webhooks not initialized`)
        return
      }

      const isFresh = (checkedAt?: Date) =>
        checkedAt !== undefined &&
        checkedAt !== null &&
        Date.now() - checkedAt.getTime() <= WebhookService.ENDPOINT_FLAG_TTL_MS

      if (!isFresh(init.endpointsCheckedAt)) {
        init = await this.refreshEndpointFlagForInitialization(init)
      }

      if (!init.hasEndpoints && isFresh(init.endpointsCheckedAt)) {
        this.logger.debug(`Skipping webhook ${eventType}: no endpoints`)
        return
      }

      await this.svix.message.create(DEFAULT_WEBHOOK_APPLICATION_ID, {
        eventType,
        payload,
        eventId,
      })

      this.logger.debug(`Sent webhook ${eventType}`)
    } catch (error) {
      this.logger.error(`Failed to send webhook ${eventType}:`, error)
      throw error
    }
  }

  async getMessageAttempts(messageId: string): Promise<unknown[]> {
    if (!this.svix) {
      throw new ServiceUnavailableException('Webhook service is not configured')
    }

    try {
      const attempts = await this.svix.messageAttempt.listByMsg(DEFAULT_WEBHOOK_APPLICATION_ID, messageId)
      return attempts.data
    } catch (error) {
      this.logger.error(`Failed to get message attempts for message ${messageId}:`, error)
      throw error
    }
  }

  isEnabled(): boolean {
    return this.svix !== null
  }

  async getAppPortalAccess(): Promise<{ token: string; url: string }> {
    if (!this.svix) {
      throw new ServiceUnavailableException('Webhook service is not configured')
    }

    try {
      const appPortalAccess = await this.svix.authentication.appPortalAccess(DEFAULT_WEBHOOK_APPLICATION_ID, {})
      this.logger.debug('Generated app portal access')
      return {
        token: appPortalAccess.token,
        url: appPortalAccess.url,
      }
    } catch (error) {
      this.logger.debug('Failed to generate app portal access:', error)
      if (typeof error === 'object' && error !== null && 'code' in error && error.code === 404) {
        throw new NotFoundException('Webhook application not found in Svix')
      }
      throw error
    }
  }
}
