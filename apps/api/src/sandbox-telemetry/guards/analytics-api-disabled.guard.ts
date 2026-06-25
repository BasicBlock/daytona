/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { CanActivate, Injectable } from '@nestjs/common'

@Injectable()
export class AnalyticsApiDisabledGuard implements CanActivate {
  canActivate(): boolean {
    return true
  }
}
