/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { DocumentBuilder } from '@nestjs/swagger'

const getOpenApiConfig = () =>
  new DocumentBuilder()
    .setTitle('Daytona')
    .addServer('http://localhost:3000')
    .setDescription('Daytona AI platform API Docs')
    .setContact('Daytona Platforms Inc.', 'https://www.daytona.io', 'support@daytona.com')
    .setVersion('1.0')
    .setLicense('Apache-2.0', 'https://www.apache.org/licenses/LICENSE-2.0')
    .build()

export { getOpenApiConfig }
