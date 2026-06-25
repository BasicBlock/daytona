/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { MigrationInterface, QueryRunner } from 'typeorm'

/**
 * Reconciliation migration that resolves drift between the database schema and the current entity definitions in the codebase.
 */
export class Migration1774438866001 implements MigrationInterface {
  name = 'Migration1774438866001'

  public async up(queryRunner: QueryRunner): Promise<void> {
    /**
     * Constraint renames due to conflict with custom naming strategy.
     */
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot_region',
      'FK_snapshot_region_snapshot',
      'snapshot_region_snapshotId_fk',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot_region',
      'FK_snapshot_region_region',
      'snapshot_region_regionId_fk',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot',
      'public.snapshot_buildInfoImageRef_fk',
      'snapshot_buildInfoSnapshotRef_fk',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'sandbox',
      'public.sandbox_buildInfoSnapshotRef_fk',
      'sandbox_buildInfoSnapshotRef_fk',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot',
      'image_organizationId_name_unique',
      'snapshot_organizationId_name_unique',
    )
    await this.renameConstraintIfExists(queryRunner, 'sandbox', 'public.sandbox_id_pk', 'sandbox_id_pk')

    /** Add missing column defaults for runner that the entity defines but the original migration omitted. */
    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "cpu" SET DEFAULT '0'`)
    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "memoryGiB" SET DEFAULT '0'`)
    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "diskGiB" SET DEFAULT '0'`)

    /**
     * Recreate roleId FKs on junction tables with NO ACTION instead of CASCADE.
     * TypeORM's onDelete: CASCADE on ManyToMany only applies to the owning side FK,
     * the inverse side (roleId) defaults to NO ACTION. The original migrations incorrectly
     * created both FKs with CASCADE. This aligns the database with TypeORM's expected schema.
     */
    if (await this.organizationRoleTablesExist(queryRunner)) {
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment_invitation" DROP CONSTRAINT IF EXISTS "organization_role_assignment_invitation_roleId_fk"`,
      )
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment" DROP CONSTRAINT IF EXISTS "organization_role_assignment_roleId_fk"`,
      )
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment_invitation" ADD CONSTRAINT "organization_role_assignment_invitation_roleId_fk" FOREIGN KEY ("roleId") REFERENCES "organization_role"("id") ON DELETE NO ACTION ON UPDATE NO ACTION`,
      )
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment" ADD CONSTRAINT "organization_role_assignment_roleId_fk" FOREIGN KEY ("roleId") REFERENCES "organization_role"("id") ON DELETE NO ACTION ON UPDATE NO ACTION`,
      )
    }
  }

  public async down(queryRunner: QueryRunner): Promise<void> {
    if (await this.organizationRoleTablesExist(queryRunner)) {
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment" DROP CONSTRAINT IF EXISTS "organization_role_assignment_roleId_fk"`,
      )
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment_invitation" DROP CONSTRAINT IF EXISTS "organization_role_assignment_invitation_roleId_fk"`,
      )
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment" ADD CONSTRAINT "organization_role_assignment_roleId_fk" FOREIGN KEY ("roleId") REFERENCES "organization_role"("id") ON DELETE CASCADE ON UPDATE CASCADE`,
      )
      await queryRunner.query(
        `ALTER TABLE "organization_role_assignment_invitation" ADD CONSTRAINT "organization_role_assignment_invitation_roleId_fk" FOREIGN KEY ("roleId") REFERENCES "organization_role"("id") ON DELETE CASCADE ON UPDATE CASCADE`,
      )
    }

    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "diskGiB" DROP DEFAULT`)
    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "memoryGiB" DROP DEFAULT`)
    await queryRunner.query(`ALTER TABLE "runner" ALTER COLUMN "cpu" DROP DEFAULT`)

    await this.renameConstraintIfExists(queryRunner, 'sandbox', 'sandbox_id_pk', 'public.sandbox_id_pk')

    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot',
      'snapshot_organizationId_name_unique',
      'image_organizationId_name_unique',
    )

    await this.renameConstraintIfExists(
      queryRunner,
      'sandbox',
      'sandbox_buildInfoSnapshotRef_fk',
      'public.sandbox_buildInfoSnapshotRef_fk',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot',
      'snapshot_buildInfoSnapshotRef_fk',
      'public.snapshot_buildInfoImageRef_fk',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot_region',
      'snapshot_region_regionId_fk',
      'FK_snapshot_region_region',
    )
    await this.renameConstraintIfExists(
      queryRunner,
      'snapshot_region',
      'snapshot_region_snapshotId_fk',
      'FK_snapshot_region_snapshot',
    )
  }

  private async renameConstraintIfExists(
    queryRunner: QueryRunner,
    tableName: string,
    oldConstraintName: string,
    newConstraintName: string,
  ): Promise<void> {
    const [{ exists }] = await queryRunner.query(
      `
        SELECT EXISTS (
          SELECT 1
          FROM pg_constraint c
          JOIN pg_class t ON t.oid = c.conrelid
          JOIN pg_namespace n ON n.oid = t.relnamespace
          WHERE n.nspname = 'public'
            AND t.relname = $1
            AND c.conname = $2
        ) AS exists
      `,
      [tableName, oldConstraintName],
    )

    if (!exists) {
      return
    }

    await queryRunner.query(
      `ALTER TABLE ${this.qualifiedName(tableName)} RENAME CONSTRAINT ${this.quoteIdentifier(
        oldConstraintName,
      )} TO ${this.quoteIdentifier(newConstraintName)}`,
    )
  }

  private async organizationRoleTablesExist(queryRunner: QueryRunner): Promise<boolean> {
    const [{ exists }] = await queryRunner.query(
      `
        SELECT (
          to_regclass('public.organization_role') IS NOT NULL
          AND to_regclass('public.organization_role_assignment') IS NOT NULL
          AND to_regclass('public.organization_role_assignment_invitation') IS NOT NULL
        ) AS exists
      `,
    )

    return exists
  }

  private qualifiedName(tableName: string): string {
    return `"public"."${tableName}"`
  }

  private quoteIdentifier(identifier: string): string {
    return `"${identifier.replace(/"/g, '""')}"`
  }
}
