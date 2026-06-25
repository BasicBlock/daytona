/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ApiSchema } from '@nestjs/swagger'
import { Runner } from '../entities/runner.entity'
import { RunnerDto } from './runner.dto'

@ApiSchema({ name: 'RunnerFull' })
export class RunnerFullDto extends RunnerDto {
  static fromRunner(runner: Runner): RunnerFullDto {
    return {
      ...RunnerDto.fromRunner(runner),
    }
  }
}
