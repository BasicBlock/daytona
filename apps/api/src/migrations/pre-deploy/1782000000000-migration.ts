/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

export class Migration1782000000000 implements MigrationInterface {
  name = 'Migration1782000000000'

  public async up(queryRunner: QueryRunner): Promise<void> {
    await this.ensureTextColumnFromOptionalSource(
      queryRunner,
      'runner',
      'target',
      'region',
      process.env.DEFAULT_TARGET_ID || 'us',
    )
    await this.ensureTextColumnFromOptionalSource(
      queryRunner,
      'sandbox',
      'target',
      'region',
      process.env.DEFAULT_TARGET_ID || 'us',
    )
    await this.ensureRunnerName(queryRunner)
    await this.normalizeDockerRegistryTypeEnum(queryRunner)
    await this.normalizeWebhookInitialization(queryRunner)

    await this.createIndexIfTableExists(queryRunner, 'runner', 'runner_target_name_unique', ['target', 'name'], true)
    await this.createIndexIfTableExists(queryRunner, 'runner', 'runner_state_unschedulable_target_index', [
      'state',
      'unschedulable',
      'target',
    ])
    await this.createIndexIfTableExists(queryRunner, 'sandbox', 'sandbox_target_idx', ['target'])
  }

  public async down(): Promise<void> {
    throw new Error('Simplifying organizations, users, and regions is irreversible')
  }

  private async ensureTextColumnFromOptionalSource(
    queryRunner: QueryRunner,
    tableName: string,
    columnName: string,
    sourceColumnName: string,
    fallbackValue: string,
  ): Promise<void> {
    if (!(await this.tableExists(queryRunner, tableName))) {
      return
    }

    if (!(await this.columnExists(queryRunner, tableName, columnName))) {
      await queryRunner.query(
        `ALTER TABLE ${this.qualifiedName('public', tableName)} ADD COLUMN ${this.quoteIdentifier(columnName)} character varying`,
      )
    }

    const sourceExpression = (await this.columnExists(queryRunner, tableName, sourceColumnName))
      ? `${this.quoteIdentifier(sourceColumnName)}::text`
      : 'NULL'

    await queryRunner.query(
      `UPDATE ${this.qualifiedName('public', tableName)} SET ${this.quoteIdentifier(columnName)} = COALESCE(${this.quoteIdentifier(columnName)}, ${sourceExpression}, $1)`,
      [fallbackValue],
    )
    await queryRunner.query(
      `ALTER TABLE ${this.qualifiedName('public', tableName)} ALTER COLUMN ${this.quoteIdentifier(columnName)} SET NOT NULL`,
    )
  }

  private async ensureRunnerName(queryRunner: QueryRunner): Promise<void> {
    if (!(await this.tableExists(queryRunner, 'runner'))) {
      return
    }

    if (!(await this.columnExists(queryRunner, 'runner', 'name'))) {
      await queryRunner.query(`ALTER TABLE "runner" ADD COLUMN "name" character varying`)
    }

    await queryRunner.query(`UPDATE "runner" SET "name" = COALESCE("name", "id"::text)`)
    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "name" SET NOT NULL`)
  }

  private async normalizeDockerRegistryTypeEnum(queryRunner: QueryRunner): Promise<void> {
    if (!(await this.tableExists(queryRunner, 'docker_registry'))) {
      return
    }

    const [{ exists }] = await queryRunner.query(
      `
        SELECT EXISTS (
          SELECT 1
          FROM pg_type t
          INNER JOIN pg_namespace n ON n.oid = t.typnamespace
          WHERE n.nspname = 'public'
            AND t.typname = 'docker_registry_registrytype_enum'
        ) AS "exists"
      `,
    )
    if (!exists) {
      return
    }

    await queryRunner.query(
      `ALTER TYPE "public"."docker_registry_registrytype_enum" RENAME TO "docker_registry_registrytype_enum_old"`,
    )
    await queryRunner.query(
      `CREATE TYPE "public"."docker_registry_registrytype_enum" AS ENUM('internal', 'custom', 'transient', 'backup')`,
    )
    await queryRunner.query(`ALTER TABLE "docker_registry" ALTER COLUMN "registryType" DROP DEFAULT`)
    await queryRunner.query(`
      ALTER TABLE "docker_registry"
      ALTER COLUMN "registryType" TYPE "public"."docker_registry_registrytype_enum"
      USING (
        CASE
          WHEN "registryType"::text IN ('organization', 'user', 'public') THEN 'custom'
          WHEN "registryType"::text = 'backup' THEN 'backup'
          WHEN "registryType"::text = 'transient' THEN 'transient'
          ELSE 'internal'
        END
      )::"public"."docker_registry_registrytype_enum"
    `)
    await queryRunner.query(`ALTER TABLE "docker_registry" ALTER COLUMN "registryType" SET DEFAULT 'internal'`)
    await queryRunner.query(`DROP TYPE "public"."docker_registry_registrytype_enum_old"`)
  }

  private async normalizeWebhookInitialization(queryRunner: QueryRunner): Promise<void> {
    if (!(await this.tableExists(queryRunner, 'webhook_initialization'))) {
      await queryRunner.query(`
        CREATE TABLE "webhook_initialization" (
          "applicationId" character varying NOT NULL,
          "svixApplicationId" character varying,
          "lastError" text,
          "retryCount" integer NOT NULL DEFAULT 0,
          "hasEndpoints" boolean NOT NULL DEFAULT false,
          "endpointsCheckedAt" TIMESTAMP WITH TIME ZONE,
          "createdAt" TIMESTAMP NOT NULL DEFAULT now(),
          "updatedAt" TIMESTAMP NOT NULL DEFAULT now(),
          CONSTRAINT "PK_webhook_initialization_applicationId" PRIMARY KEY ("applicationId")
        )
      `)
      return
    }

    if (
      (await this.columnExists(queryRunner, 'webhook_initialization', 'organizationId')) &&
      !(await this.columnExists(queryRunner, 'webhook_initialization', 'applicationId'))
    ) {
      await queryRunner.query(`ALTER TABLE "webhook_initialization" RENAME COLUMN "organizationId" TO "applicationId"`)
    }
  }

  private async createIndexIfTableExists(
    queryRunner: QueryRunner,
    tableName: string,
    indexName: string,
    columnNames: string[],
    unique = false,
  ): Promise<void> {
    if (!(await this.tableExists(queryRunner, tableName))) {
      return
    }

    for (const columnName of columnNames) {
      if (!(await this.columnExists(queryRunner, tableName, columnName))) {
        return
      }
    }

    const uniqueClause = unique ? 'UNIQUE ' : ''
    const columns = columnNames.map((columnName) => this.quoteIdentifier(columnName)).join(', ')
    await queryRunner.query(
      `CREATE ${uniqueClause}INDEX IF NOT EXISTS ${this.quoteIdentifier(indexName)} ON ${this.qualifiedName('public', tableName)} (${columns})`,
    )
  }

  private async columnExists(queryRunner: QueryRunner, tableName: string, columnName: string): Promise<boolean> {
    const [{ exists }] = await queryRunner.query(
      `
        SELECT EXISTS (
          SELECT 1
          FROM information_schema.columns
          WHERE table_schema = 'public'
            AND table_name = $1
            AND column_name = $2
        ) AS "exists"
      `,
      [tableName, columnName],
    )

    return exists
  }

  private async tableExists(queryRunner: QueryRunner, tableName: string): Promise<boolean> {
    const [{ exists }] = await queryRunner.query(
      `
        SELECT EXISTS (
          SELECT 1
          FROM information_schema.tables
          WHERE table_schema = 'public'
            AND table_name = $1
        ) AS "exists"
      `,
      [tableName],
    )

    return exists
  }

  private qualifiedName(schemaName: string, objectName: string): string {
    return `${this.quoteIdentifier(schemaName)}.${this.quoteIdentifier(objectName)}`
  }

  private quoteIdentifier(identifier: string): string {
    return `"${identifier.replace(/"/g, '""')}"`
  }
}
