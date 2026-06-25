/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ApiProperty, ApiSchema } from '@nestjs/swagger'
import { Runner } from '../entities/runner.entity'

@ApiSchema({ name: 'CreateRunnerResponse' })
export class CreateRunnerResponseDto {
  @ApiProperty({
    description: 'The ID of the runner',
    example: 'runner123',
  })
  id: string

  static fromRunner(runner: Runner): CreateRunnerResponseDto {
    return {
      id: runner.id,
    }
  }
}
