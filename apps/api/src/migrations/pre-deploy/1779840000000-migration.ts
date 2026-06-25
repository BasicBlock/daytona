/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

export class Migration1779840000000 implements MigrationInterface {
  name = 'Migration1779840000000'

  public async up(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "region_quota" ADD COLUMN IF NOT EXISTS "total_gpu_quota" integer NOT NULL DEFAULT 0`,
    )
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "region_quota" ADD COLUMN IF NOT EXISTS "max_cpu_per_gpu_sandbox" integer`,
    )
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "region_quota" ADD COLUMN IF NOT EXISTS "max_memory_per_gpu_sandbox" integer`,
    )
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "region_quota" ADD COLUMN IF NOT EXISTS "max_disk_per_gpu_sandbox" integer`,
    )
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`ALTER TABLE IF EXISTS "region_quota" DROP COLUMN IF EXISTS "max_disk_per_gpu_sandbox"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "region_quota" DROP COLUMN IF EXISTS "max_memory_per_gpu_sandbox"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "region_quota" DROP COLUMN IF EXISTS "max_cpu_per_gpu_sandbox"`)
    await queryRunner.query(`ALTER TABLE IF EXISTS "region_quota" DROP COLUMN IF EXISTS "total_gpu_quota"`)
  }
}
