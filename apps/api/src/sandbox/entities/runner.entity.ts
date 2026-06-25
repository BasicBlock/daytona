/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Column, CreateDateColumn, Entity, Index, PrimaryGeneratedColumn, Unique, UpdateDateColumn } from 'typeorm'
import { SandboxClass } from '../enums/sandbox-class.enum'
import { GpuType } from '../enums/gpu-type.enum'
import { RunnerState } from '../enums/runner-state.enum'
import { RunnerServiceInfo } from '../common/runner-service-info'

@Entity()
@Unique(['target', 'name'])
// TODO: extend with `sandboxClass` once multi-class runner pools become common.
@Index(['state', 'unschedulable', 'target'])
@Index('runner_tags_gin_idx', { synchronize: false })
export class Runner {
  @PrimaryGeneratedColumn('uuid')
  id: string

  @Column({
    nullable: true,
  })
  domain: string | null

  @Column({
    nullable: true,
  })
  apiUrl: string | null

  @Column({
    nullable: true,
  })
  proxyUrl: string | null

  @Column({
    type: 'float',
    default: 0,
  })
  cpu: number

  @Column({
    type: 'float',
    default: 0,
  })
  memoryGiB: number

  @Column({
    type: 'float',
    default: 0,
  })
  diskGiB: number

  @Column({
    nullable: true,
  })
  gpu: number | null

  @Column({
    type: 'character varying',
    nullable: true,
  })
  gpuType: GpuType | null

  @Column({
    type: 'character varying',
    default: SandboxClass.LINUX_VM,
  })
  sandboxClass: SandboxClass = SandboxClass.LINUX_VM

  @Column({
    type: 'float',
    default: 0,
  })
  currentCpuLoadAverage: number

  @Column({
    type: 'float',
    default: 0,
  })
  currentCpuUsagePercentage: number

  @Column({
    type: 'float',
    default: 0,
  })
  currentMemoryUsagePercentage: number

  @Column({
    type: 'float',
    default: 0,
  })
  currentDiskUsagePercentage: number

  @Column({
    type: 'float',
    default: 0,
  })
  currentAllocatedCpu: number

  @Column({
    type: 'float',
    default: 0,
  })
  currentAllocatedMemoryGiB: number

  @Column({
    type: 'float',
    default: 0,
  })
  currentAllocatedDiskGiB: number

  @Column({
    default: 0,
  })
  currentSnapshotCount: number

  @Column({
    default: 0,
  })
  currentStartedSandboxes: number

  @Column({
    default: 0,
  })
  availabilityScore: number

  @Column()
  target: string

  @Column()
  name: string

  @Column({
    type: 'enum',
    enum: RunnerState,
    default: RunnerState.INITIALIZING,
  })
  state: RunnerState

  @Column({
    default: 'v0.0.0-dev',
    nullable: true,
  })
  appVersion: string | null

  @Column({
    default: '0',
  })
  apiVersion: string

  @Column({
    nullable: true,
    type: 'timestamp with time zone',
  })
  lastChecked: Date

  @Column({
    default: false,
  })
  unschedulable: boolean

  @Column({
    default: false,
  })
  draining: boolean

  @Column({
    type: 'text',
    array: true,
    default: () => "'{}'",
  })
  tags: string[]

  @Column({
    type: 'jsonb',
    nullable: true,
    default: null,
  })
  serviceHealth: RunnerServiceInfo[] | null

  @CreateDateColumn({
    type: 'timestamp with time zone',
  })
  createdAt: Date

  @UpdateDateColumn({
    type: 'timestamp with time zone',
  })
  updatedAt: Date

  constructor(params?: {
    target: string
    name: string
    apiVersion: string
    cpu?: number
    memoryGiB?: number
    diskGiB?: number
    domain?: string | null
    apiUrl?: string
    proxyUrl?: string
    appVersion?: string | null
    tags?: string[]
    sandboxClass?: SandboxClass
  }) {
    if (!params) return
    this.target = params.target
    this.name = params.name
    this.cpu = params.cpu ?? 0
    this.memoryGiB = params.memoryGiB ?? 0
    this.diskGiB = params.diskGiB ?? 0
    this.domain = params.domain ?? null
    this.apiUrl = params.apiUrl
    this.proxyUrl = params.proxyUrl
    this.sandboxClass = params.sandboxClass ?? SandboxClass.LINUX_VM
    this.apiVersion = params.apiVersion
    this.appVersion = params.appVersion ?? null
    this.gpu = null
    this.gpuType = null
    this.tags = params.tags ?? []
  }
}
