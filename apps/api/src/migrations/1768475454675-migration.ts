/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

export class Migration1768475454675 implements MigrationInterface {
  name = 'Migration1768475454675'

  public async up(queryRunner: QueryRunner): Promise<void> {
    if (!(await this.tableExists(queryRunner, 'region'))) {
      return
    }

    await queryRunner.query(
      `CREATE INDEX IF NOT EXISTS "idx_region_custom" ON "region" ("organizationId") WHERE "regionType" = 'custom'`,
    )
    await queryRunner.query(
      `CREATE UNIQUE INDEX IF NOT EXISTS "region_sshGatewayApiKeyHash_unique" ON "region" ("sshGatewayApiKeyHash") WHERE "sshGatewayApiKeyHash" IS NOT NULL`,
    )
    await queryRunner.query(
      `CREATE UNIQUE INDEX IF NOT EXISTS "region_proxyApiKeyHash_unique" ON "region" ("proxyApiKeyHash") WHERE "proxyApiKeyHash" IS NOT NULL`,
    )
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    await queryRunner.query(`DROP INDEX IF EXISTS "public"."region_proxyApiKeyHash_unique"`)
    await queryRunner.query(`DROP INDEX IF EXISTS "public"."region_sshGatewayApiKeyHash_unique"`)
    await queryRunner.query(`DROP INDEX IF EXISTS "public"."idx_region_custom"`)
  }

  private async tableExists(queryRunner: QueryRunner, tableName: string): Promise<boolean> {
    const [{ exists }] = await queryRunner.query(`SELECT to_regclass($1) IS NOT NULL AS exists`, [
      `public.${tableName}`,
    ])

    return exists
  }
}
