/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { RegistryType } from './../../docker-registry/enums/registry-type.enum'
import { Column, CreateDateColumn, Entity, Index, PrimaryGeneratedColumn, UpdateDateColumn } from 'typeorm'

@Entity()
@Index(['registryType', 'isDefault'])
export class DockerRegistry {
  @PrimaryGeneratedColumn('uuid')
  id: string

  @Column()
  name: string

  @Column()
  url: string

  @Column()
  username: string

  @Column()
  password: string

  @Column({ default: false })
  isDefault: boolean

  @Column({ default: false })
  isFallback: boolean

  @Column({ default: '' })
  project: string

  @Column({
    type: 'enum',
    enum: RegistryType,
    default: RegistryType.INTERNAL,
  })
  registryType: RegistryType

  @CreateDateColumn({
    type: 'timestamp with time zone',
  })
  createdAt: Date

  @UpdateDateColumn({
    type: 'timestamp with time zone',
  })
  updatedAt: Date
}
