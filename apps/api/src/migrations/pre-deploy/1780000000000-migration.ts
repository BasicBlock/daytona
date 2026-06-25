/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

export class Migration1780000000000 implements MigrationInterface {
  name = 'Migration1780000000000'

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TABLE IF EXISTS "snapshot" ADD COLUMN IF NOT EXISTS "gpu_type" character varying`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "sandbox" ADD COLUMN IF NOT EXISTS "gpu_type" character varying`)
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "sandbox_usage_periods" ADD COLUMN IF NOT EXISTS "gpu_type" character varying`,
    )
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "sandbox_usage_periods_archive" ADD COLUMN IF NOT EXISTS "gpu_type" character varying`,
    )
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "region_quota" ADD COLUMN IF NOT EXISTS "allowed_gpu_types" text array`,
    )
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TABLE IF EXISTS "region_quota" DROP COLUMN IF EXISTS "allowed_gpu_types"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "sandbox_usage_periods_archive" DROP COLUMN IF EXISTS "gpu_type"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "sandbox_usage_periods" DROP COLUMN IF EXISTS "gpu_type"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "sandbox" DROP COLUMN IF EXISTS "gpu_type"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "snapshot" DROP COLUMN IF EXISTS "gpu_type"`)
  }
}
