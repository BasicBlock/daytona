/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ApiProperty, ApiSchema } from '@nestjs/swagger'

@ApiSchema({ name: 'OtelForwardingConfig' })
export class OtelForwardingConfigDto {
  @ApiProperty({ description: 'OTLP HTTP endpoint for forwarding sandbox telemetry' })
  endpoint: string

  @ApiProperty({
    description: 'HTTP headers to include when forwarding sandbox telemetry',
    type: 'object',
    additionalProperties: { type: 'string' },
  })
  headers: Record<string, string>
}
