/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Controller, Get, HttpCode } from '@nestjs/common'
import { ApiTags, ApiOperation, ApiResponse } from '@nestjs/swagger'
import { ObjectStorageService } from '../services/object-storage.service'
import { StorageAccessDto } from '../../sandbox/dto/storage-access-dto'

@Controller('object-storage')
@ApiTags('object-storage')
export class ObjectStorageController {
  constructor(private readonly objectStorageService: ObjectStorageService) {}

  @Get('push-access')
  @HttpCode(200)
  @ApiOperation({
    summary: 'Get temporary storage access for pushing objects',
    operationId: 'getPushAccess',
  })
  @ApiResponse({
    status: 200,
    description: 'Temporary storage access has been generated',
    type: StorageAccessDto,
  })
  async getPushAccess(): Promise<StorageAccessDto> {
    return this.objectStorageService.getPushAccess()
  }
}
