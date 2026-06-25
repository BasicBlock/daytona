/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

export class Migration1782000000001 implements MigrationInterface {
  name = 'Migration1782000000001'

  public async up(queryRunner: QueryRunner): Promise<void> {
    await this.dropForeignKeysReferencing(queryRunner, 'organization')
    await this.dropForeignKeysReferencing(queryRunner, 'user')
    await this.dropForeignKeysReferencing(queryRunner, 'region')

    await this.dropTables(queryRunner, [
      'organization_role_assignment_invitation',
      'organization_role_assignment',
      'organization_invitation',
      'organization_user',
      'organization_role',
      'api_key',
      'audit_log',
      'sandbox_usage_periods',
      'sandbox_usage_periods_archive',
      'workspace_usage_periods',
      'snapshot_region',
      'region_quota',
      'region',
      'organization',
      'user',
    ])

    const columnsToDrop: Record<string, string[]> = {
      docker_registry: ['organizationId', 'userId', 'region'],
      image: ['organizationId', 'userId'],
      runner: ['region', 'apiKey'],
      sandbox: ['organizationId', 'region'],
      snapshot: ['organizationId'],
      volume: ['organizationId'],
      workspace: ['organizationId', 'userId', 'region'],
    }

    for (const [tableName, columnNames] of Object.entries(columnsToDrop)) {
      for (const columnName of columnNames) {
        await this.dropConstraintsContainingColumn(queryRunner, tableName, columnName)
      }
    }

    for (const [tableName, columnNames] of Object.entries(columnsToDrop)) {
      for (const columnName of columnNames) {
        await this.dropIndexesContainingColumn(queryRunner, tableName, columnName)
      }
    }

    for (const [tableName, columnNames] of Object.entries(columnsToDrop)) {
      for (const columnName of columnNames) {
        await this.dropColumnIfExists(queryRunner, tableName, columnName)
      }
    }

    await this.dropTypes(queryRunner, [
      'api_key_permissions_enum',
      'organization_invitation_role_enum',
      'organization_invitation_status_enum',
      'organization_role_permissions_enum',
      'organization_user_role_enum',
      'region_regiontype_enum',
      'runner_region_enum',
      'sandbox_region_enum',
      'user_role_enum',
      'workspace_region_enum',
    ])
  }

  public async down(): Promise<void> {
    throw new Error('Dropping organizations, users, and regions is irreversible')
  }

  private async dropTables(queryRunner: QueryRunner, tableNames: string[]): Promise<void> {
    for (const tableName of tableNames) {
      await queryRunner.query(`DROP TABLE IF EXISTS ${this.qualifiedName('public', tableName)} CASCADE`)
    }
  }

  private async dropTypes(queryRunner: QueryRunner, typeNames: string[]): Promise<void> {
    for (const typeName of typeNames) {
      await queryRunner.query(`DROP TYPE IF EXISTS ${this.qualifiedName('public', typeName)}`)
    }
  }

  private async dropColumnIfExists(queryRunner: QueryRunner, tableName: string, columnName: string): Promise<void> {
    if (!(await this.columnExists(queryRunner, tableName, columnName))) {
      return
    }

    await queryRunner.query(
      `ALTER TABLE ${this.qualifiedName('public', tableName)} DROP COLUMN ${this.quoteIdentifier(columnName)}`,
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

  private async dropForeignKeysReferencing(queryRunner: QueryRunner, referencedTableName: string): Promise<void> {
    const foreignKeys: { schemaName: string; tableName: string; constraintName: string }[] = await queryRunner.query(
      `
        SELECT
          schema_namespace.nspname AS "schemaName",
          source_table.relname AS "tableName",
          constraint_record.conname AS "constraintName"
        FROM pg_constraint constraint_record
        INNER JOIN pg_class source_table ON source_table.oid = constraint_record.conrelid
        INNER JOIN pg_namespace schema_namespace ON schema_namespace.oid = source_table.relnamespace
        WHERE constraint_record.contype = 'f'
          AND constraint_record.confrelid = to_regclass($1)
      `,
      [`public.${this.quoteIdentifier(referencedTableName)}`],
    )

    for (const foreignKey of foreignKeys) {
      await queryRunner.query(
        `ALTER TABLE ${this.qualifiedName(foreignKey.schemaName, foreignKey.tableName)} DROP CONSTRAINT IF EXISTS ${this.quoteIdentifier(foreignKey.constraintName)}`,
      )
    }
  }

  private async dropConstraintsContainingColumn(
    queryRunner: QueryRunner,
    tableName: string,
    columnName: string,
  ): Promise<void> {
    const constraints: { schemaName: string; constraintName: string }[] = await queryRunner.query(
      `
        SELECT DISTINCT
          schema_namespace.nspname AS "schemaName",
          constraint_record.conname AS "constraintName"
        FROM pg_constraint constraint_record
        INNER JOIN pg_class source_table ON source_table.oid = constraint_record.conrelid
        INNER JOIN pg_namespace schema_namespace ON schema_namespace.oid = source_table.relnamespace
        INNER JOIN pg_attribute attribute_record
          ON attribute_record.attrelid = source_table.oid
          AND attribute_record.attnum = ANY(constraint_record.conkey)
        WHERE schema_namespace.nspname = 'public'
          AND source_table.relname = $1
          AND attribute_record.attname = $2
      `,
      [tableName, columnName],
    )

    for (const constraint of constraints) {
      await queryRunner.query(
        `ALTER TABLE ${this.qualifiedName(constraint.schemaName, tableName)} DROP CONSTRAINT IF EXISTS ${this.quoteIdentifier(constraint.constraintName)}`,
      )
    }
  }

  private async dropIndexesContainingColumn(
    queryRunner: QueryRunner,
    tableName: string,
    columnName: string,
  ): Promise<void> {
    const indexes: { schemaName: string; indexName: string }[] = await queryRunner.query(
      `
        SELECT DISTINCT
          index_namespace.nspname AS "schemaName",
          index_class.relname AS "indexName"
        FROM pg_index index_record
        INNER JOIN pg_class source_table ON source_table.oid = index_record.indrelid
        INNER JOIN pg_namespace source_namespace ON source_namespace.oid = source_table.relnamespace
        INNER JOIN pg_class index_class ON index_class.oid = index_record.indexrelid
        INNER JOIN pg_namespace index_namespace ON index_namespace.oid = index_class.relnamespace
        INNER JOIN pg_attribute attribute_record
          ON attribute_record.attrelid = source_table.oid
          AND attribute_record.attnum = ANY(index_record.indkey)
        LEFT JOIN pg_constraint constraint_record ON constraint_record.conindid = index_record.indexrelid
        WHERE source_namespace.nspname = 'public'
          AND source_table.relname = $1
          AND attribute_record.attname = $2
          AND constraint_record.oid IS NULL
      `,
      [tableName, columnName],
    )

    for (const index of indexes) {
      await queryRunner.query(`DROP INDEX IF EXISTS ${this.qualifiedName(index.schemaName, index.indexName)}`)
    }
  }

  private qualifiedName(schemaName: string, objectName: string): string {
    return `${this.quoteIdentifier(schemaName)}.${this.quoteIdentifier(objectName)}`
  }

  private quoteIdentifier(identifier: string): string {
    return `"${identifier.replace(/"/g, '""')}"`
  }
}
