/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

export class Migration1766415256696 implements MigrationInterface {
  name = 'Migration1766415256696'

  public async up(queryRunner: QueryRunner): Promise<void> {
    // region snapshot manager field
    await queryRunner.query(
      `ALTER TABLE IF EXISTS "region" ADD COLUMN IF NOT EXISTS "snapshotManagerUrl" character varying`,
    )

    // docker registry indexes
    await queryRunner.query(
      `CREATE INDEX IF NOT EXISTS "docker_registry_registryType_isDefault_index" ON "docker_registry" ("registryType", "isDefault") `,
    )
    await queryRunner.query(
      `CREATE INDEX IF NOT EXISTS "docker_registry_region_registryType_index" ON "docker_registry" ("region", "registryType") `,
    )
    await queryRunner.query(
      `CREATE INDEX IF NOT EXISTS "docker_registry_organizationId_registryType_index" ON "docker_registry" ("organizationId", "registryType") `,
    )
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    // drop region snapshot manager field
    await queryRunner.query(`ALTER TABLE IF EXISTS "region" DROP COLUMN IF EXISTS "snapshotManagerUrl"`)

    // drop docker registry indexes
    await queryRunner.query(`DROP INDEX IF EXISTS "public"."docker_registry_organizationId_registryType_index"`)
    await queryRunner.query(`DROP INDEX IF EXISTS "public"."docker_registry_region_registryType_index"`)
    await queryRunner.query(`DROP INDEX IF EXISTS "public"."docker_registry_registryType_isDefault_index"`)
  }
}
