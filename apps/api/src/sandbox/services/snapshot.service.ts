/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { BadRequestException, ConflictException, Injectable, Logger, NotFoundException } from '@nestjs/common'
import { Cron, CronExpression } from '@nestjs/schedule'
import { EventEmitter2, OnEvent } from '@nestjs/event-emitter'
import { InjectRepository } from '@nestjs/typeorm'
import { In, LessThan, Like, Not, Raw, Repository, ILike, FindOptionsWhere } from 'typeorm'
import { v4 as uuidv4, validate as isUUID } from 'uuid'
import { SnapshotRepository } from '../repositories/snapshot.repository'
import { Snapshot } from '../entities/snapshot.entity'
import { BuildInfo, generateBuildInfoHash as generateBuildSnapshotRef } from '../entities/build-info.entity'
import { SnapshotRunner } from '../entities/snapshot-runner.entity'
import { SnapshotState } from '../enums/snapshot-state.enum'
import { SnapshotRunnerState } from '../enums/snapshot-runner-state.enum'
import { SandboxState } from '../enums/sandbox-state.enum'
import { SandboxClass } from '../enums/sandbox-class.enum'
import { RunnerState } from '../enums/runner-state.enum'
import { CreateSnapshotDto } from '../dto/create-snapshot.dto'
import { PaginatedList } from '../../common/interfaces/paginated-list.interface'
import { SnapshotSortDirection, SnapshotSortField } from '../dto/list-snapshots-query.dto'
import { DockerRegistryService } from '../../docker-registry/services/docker-registry.service'
import { SnapshotEvents } from '../constants/snapshot-events'
import { SnapshotCreatedEvent } from '../events/snapshot-created.event'
import { SnapshotActivatedEvent } from '../events/snapshot-activated.event'
import { RunnerService } from './runner.service'
import { TypedConfigService } from '../../config/typed-config.service'
import { SandboxRepository } from '../repositories/sandbox.repository'
import { SandboxEvents } from '../constants/sandbox-events.constants'
import { SandboxCreatedEvent } from '../events/sandbox-create.event'
import { RunnerEvents } from '../constants/runner-events'
import { RunnerDeletedEvent } from '../events/runner-deleted.event'
import { OnAsyncEvent } from '../../common/decorators/on-async-event.decorator'
import { LogExecution } from '../../common/decorators/log-execution.decorator'
import { WithInstrumentation } from '../../common/decorators/otel.decorator'
import { RunnerAdapterFactory } from '../runner-adapter/runnerAdapter'
import {
  persistSnapshotFromSandbox,
  PersistSnapshotFromSandboxParams,
} from '../utils/persist-snapshot-from-sandbox.util'
import { getRunnerSandboxClass } from '../utils/sandbox-class.util'
import { GpuType } from '../enums/gpu-type.enum'
import { SnapshotStateError } from '../errors/snapshot-state-error'

const IMAGE_NAME_REGEX = /^[a-zA-Z0-9_.\-:]+(\/[a-zA-Z0-9_.\-:]+)*(@sha256:[a-f0-9]{64})?$/

@Injectable()
export class SnapshotService {
  private readonly logger = new Logger(SnapshotService.name)

  constructor(
    private readonly sandboxRepository: SandboxRepository,
    private readonly snapshotRepository: SnapshotRepository,
    @InjectRepository(BuildInfo)
    private readonly buildInfoRepository: Repository<BuildInfo>,
    @InjectRepository(SnapshotRunner)
    private readonly snapshotRunnerRepository: Repository<SnapshotRunner>,
    private readonly runnerService: RunnerService,
    private readonly runnerAdapterFactory: RunnerAdapterFactory,
    private readonly dockerRegistryService: DockerRegistryService,
    private readonly eventEmitter: EventEmitter2,
    private readonly configService: TypedConfigService,
  ) {}

  private validateImageName(name: string): string | null {
    if (name.includes('@sha256:')) {
      const [imageName, digest] = name.split('@sha256:')
      if (!imageName || !digest || !/^[a-f0-9]{64}$/.test(digest)) {
        return 'Invalid digest format. Must be image@sha256:64_hex_characters'
      }
      return null
    }

    if (!name.includes(':') || name.endsWith(':') || /:\s*$/.test(name)) {
      return 'Image name must include a tag (e.g., ubuntu:22.04) or digest (@sha256:...)'
    }

    if (name.endsWith(':latest')) {
      return 'Images with tag ":latest" are not allowed'
    }

    if (!IMAGE_NAME_REGEX.test(name)) {
      return 'Invalid image name format. Must be lowercase, may contain digits, dots, dashes, and single slashes between components'
    }

    return null
  }

  private validateSnapshotName(name: string): string | null {
    return IMAGE_NAME_REGEX.test(name)
      ? null
      : 'Invalid snapshot name format. May contain letters, digits, dots, colons, and dashes'
  }

  private processEntrypoint(entrypoint?: string[]): string[] | undefined {
    const filtered = entrypoint?.filter((cmd) => cmd?.trim().length > 0)
    return filtered?.length ? filtered : undefined
  }

  private resolveGpuType(gpu?: number, gpuType?: GpuType[]): GpuType | null {
    return gpu && gpu > 0 ? (gpuType?.[0] ?? null) : null
  }

  private async assertHasSchedulableRunner(target: string, sandboxClass: SandboxClass): Promise<void> {
    const hasRunner = await this.runnerService.hasSchedulableRunner(target, getRunnerSandboxClass(sandboxClass))
    if (!hasRunner) {
      throw new BadRequestException(
        `No runners are configured for target '${target}' and sandbox class '${sandboxClass}'.`,
      )
    }
  }

  async createFromPull(createSnapshotDto: CreateSnapshotDto): Promise<Snapshot> {
    if (!createSnapshotDto.imageName) {
      throw new BadRequestException('Must specify an image name')
    }

    const nameValidationError = this.validateSnapshotName(createSnapshotDto.name)
    if (nameValidationError) {
      throw new BadRequestException(nameValidationError)
    }

    const imageValidationError = this.validateImageName(createSnapshotDto.imageName)
    if (imageValidationError) {
      throw new BadRequestException(imageValidationError)
    }

    const sandboxClass = createSnapshotDto.sandboxClass ?? this.configService.getOrThrow('defaultSandboxClass')
    if (sandboxClass === SandboxClass.WINDOWS) {
      throw new BadRequestException(
        'Windows snapshots cannot be created via this endpoint; they are produced by snapshot-from-sandbox flows.',
      )
    }

    const target = this.configService.getOrThrow('defaultTarget.id')
    await this.assertHasSchedulableRunner(target, sandboxClass)

    const snapshot = this.snapshotRepository.create({
      id: uuidv4(),
      name: createSnapshotDto.name,
      imageName: createSnapshotDto.imageName,
      ref: createSnapshotDto.imageName,
      state: SnapshotState.ACTIVE,
      entrypoint: this.processEntrypoint(createSnapshotDto.entrypoint),
      cpu: createSnapshotDto.cpu ?? 1,
      gpu: createSnapshotDto.gpu ?? 0,
      gpuType: this.resolveGpuType(createSnapshotDto.gpu, createSnapshotDto.gpuType),
      mem: createSnapshotDto.memory ?? 1,
      disk: createSnapshotDto.disk ?? 3,
      sandboxClass,
      lastUsedAt: new Date(),
    })

    try {
      const insertedSnapshot = await this.snapshotRepository.insert(snapshot)
      this.eventEmitter.emit(SnapshotEvents.CREATED, new SnapshotCreatedEvent(insertedSnapshot))
      return insertedSnapshot
    } catch (error) {
      if ((error as { code?: string }).code === '23505') {
        throw new ConflictException(`Snapshot with name "${createSnapshotDto.name}" already exists`)
      }
      throw error
    }
  }

  async createFromBuildInfo(createSnapshotDto: CreateSnapshotDto): Promise<Snapshot> {
    const nameValidationError = this.validateSnapshotName(createSnapshotDto.name)
    if (nameValidationError) {
      throw new BadRequestException(nameValidationError)
    }

    if (!createSnapshotDto.buildInfo?.dockerfileContent) {
      throw new BadRequestException('Must specify build information')
    }

    const sandboxClass = createSnapshotDto.sandboxClass ?? this.configService.getOrThrow('defaultSandboxClass')
    if (sandboxClass === SandboxClass.WINDOWS) {
      throw new BadRequestException(
        'Windows snapshots cannot be created via this endpoint; they are produced by snapshot-from-sandbox flows.',
      )
    }

    const target = this.configService.getOrThrow('defaultTarget.id')
    await this.assertHasSchedulableRunner(target, sandboxClass)

    const buildSnapshotRef = generateBuildSnapshotRef(
      createSnapshotDto.buildInfo.dockerfileContent,
      createSnapshotDto.buildInfo.contextHashes,
    )
    let buildInfo = await this.buildInfoRepository.findOne({ where: { snapshotRef: buildSnapshotRef } })
    if (buildInfo) {
      await this.buildInfoRepository.update(buildInfo.snapshotRef, { lastUsedAt: new Date() })
    } else {
      buildInfo = this.buildInfoRepository.create(createSnapshotDto.buildInfo)
      await this.buildInfoRepository.save(buildInfo)
    }

    const internalRegistry = await this.dockerRegistryService.getAvailableInternalRegistry(target)
    const ref = internalRegistry
      ? `${internalRegistry.url.replace(/^(https?:\/\/)/, '')}/${internalRegistry.project || 'daytona'}/${buildSnapshotRef}`
      : buildSnapshotRef

    const runner = await this.getInitialSnapshotRunner(
      target,
      sandboxClass,
      createSnapshotDto.gpu ?? 0,
      this.resolveGpuType(createSnapshotDto.gpu, createSnapshotDto.gpuType),
      true,
    )

    const snapshot = this.snapshotRepository.create({
      id: uuidv4(),
      name: createSnapshotDto.name,
      imageName: '',
      ref,
      buildInfo,
      state: SnapshotState.BUILDING,
      entrypoint: this.processEntrypoint(
        this.getEntrypointFromDockerfile(createSnapshotDto.buildInfo.dockerfileContent),
      ),
      cpu: createSnapshotDto.cpu ?? 1,
      gpu: createSnapshotDto.gpu ?? 0,
      gpuType: this.resolveGpuType(createSnapshotDto.gpu, createSnapshotDto.gpuType),
      mem: createSnapshotDto.memory ?? 1,
      disk: createSnapshotDto.disk ?? 3,
      sandboxClass,
      lastUsedAt: new Date(),
      initialRunnerId: runner.id,
    })

    try {
      const insertedSnapshot = await this.snapshotRepository.insert(snapshot)
      await this.startInitialSnapshotBuild(insertedSnapshot, runner, target)
      this.eventEmitter.emit(SnapshotEvents.CREATED, new SnapshotCreatedEvent(insertedSnapshot))
      return insertedSnapshot
    } catch (error) {
      if ((error as { code?: string }).code === '23505') {
        throw new ConflictException(`Snapshot with name "${createSnapshotDto.name}" already exists`)
      }
      throw error
    }
  }

  private async getInitialSnapshotRunner(
    target: string,
    sandboxClass: SandboxClass,
    gpu: number,
    gpuType: GpuType | null,
    isBuild: boolean,
  ) {
    const excludedRunnerIds = isBuild
      ? await this.runnerService.getRunnersWithMultipleSnapshotsBuilding()
      : await this.runnerService.getRunnersWithMultipleSnapshotsPulling()
    const availabilityScoreThreshold =
      this.configService.getOrThrow('runnerScore.thresholds.availability') +
      this.configService.getOrThrow('runnerScore.thresholds.initialRunnerScoreAddon')

    return this.runnerService.getRandomAvailableRunner({
      targets: [target],
      sandboxClass: getRunnerSandboxClass(sandboxClass),
      excludedRunnerIds,
      availabilityScoreThreshold,
      gpu,
      gpuType,
    })
  }

  private async startInitialSnapshotBuild(
    snapshot: Snapshot,
    runner: Awaited<ReturnType<SnapshotService['getInitialSnapshotRunner']>>,
    target: string,
  ): Promise<void> {
    if (!snapshot.buildInfo) {
      throw new BadRequestException('Snapshot build information is missing')
    }

    await this.runnerService.createSnapshotRunnerEntry(
      runner.id,
      snapshot.buildInfo.snapshotRef,
      SnapshotRunnerState.BUILDING_SNAPSHOT,
    )

    const runnerAdapter = await this.runnerAdapterFactory.create(runner)
    const registry = (await this.dockerRegistryService.getAvailableInternalRegistry(target)) ?? undefined
    const sourceRegistries = await this.dockerRegistryService.getSourceRegistriesForDockerfile(
      snapshot.buildInfo.dockerfileContent,
    )

    try {
      await runnerAdapter.buildSnapshot(
        snapshot.buildInfo,
        sourceRegistries.length > 0 ? sourceRegistries : undefined,
        registry,
        registry !== undefined,
      )
      void this.pollInitialSnapshotBuild(snapshot.id, runner.id, snapshot.buildInfo.snapshotRef, snapshot.ref).catch(
        (error) => this.logger.error(`Error polling snapshot build ${snapshot.id}:`, error),
      )
    } catch (error) {
      await this.markInitialSnapshotBuildFailed(snapshot, runner.id, snapshot.buildInfo.snapshotRef, error)
      throw error
    }
  }

  private async pollInitialSnapshotBuild(
    snapshotId: string,
    runnerId: string,
    buildSnapshotRef: string,
    snapshotRef?: string,
  ): Promise<void> {
    const timeoutMs = 60 * 60 * 1000
    const pollIntervalMs = 5 * 1000
    const startedAt = Date.now()
    const runner = await this.runnerService.findOneOrFail(runnerId)
    const runnerAdapter = await this.runnerAdapterFactory.create(runner)

    while (Date.now() - startedAt < timeoutMs) {
      try {
        const snapshotInfo = await runnerAdapter.getSnapshotInfo(buildSnapshotRef)
        const snapshot = await this.snapshotRepository.findOne({ where: { id: snapshotId } })
        if (!snapshot) {
          return
        }

        await this.snapshotRunnerRepository.update(
          { runnerId, snapshotRef: buildSnapshotRef },
          { state: SnapshotRunnerState.READY, errorReason: null },
        )

        if (snapshotRef && snapshotRef !== buildSnapshotRef) {
          await this.runnerService.createSnapshotRunnerEntry(runnerId, snapshotRef, SnapshotRunnerState.READY)
        }

        await this.snapshotRepository.update(snapshot.id, {
          updateData: {
            state: SnapshotState.ACTIVE,
            errorReason: null,
            size: snapshotInfo.sizeGB,
            entrypoint: snapshotInfo.entrypoint?.length ? snapshotInfo.entrypoint : snapshot.entrypoint,
            lastUsedAt: new Date(),
          },
          entity: snapshot,
        })
        return
      } catch (error) {
        if (error instanceof SnapshotStateError) {
          const snapshot = await this.snapshotRepository.findOne({ where: { id: snapshotId } })
          if (snapshot) {
            await this.markInitialSnapshotBuildFailed(snapshot, runnerId, buildSnapshotRef, error)
          }
          return
        }
      }

      await new Promise((resolve) => setTimeout(resolve, pollIntervalMs))
    }

    const snapshot = await this.snapshotRepository.findOne({ where: { id: snapshotId } })
    if (snapshot) {
      await this.markInitialSnapshotBuildFailed(
        snapshot,
        runnerId,
        buildSnapshotRef,
        new Error('Timeout while building snapshot'),
      )
    }
  }

  private async markInitialSnapshotBuildFailed(
    snapshot: Snapshot,
    runnerId: string,
    snapshotRef: string,
    error: unknown,
  ): Promise<void> {
    const message = error instanceof Error ? error.message : String(error)
    await this.snapshotRunnerRepository.update(
      { runnerId, snapshotRef },
      { state: SnapshotRunnerState.ERROR, errorReason: message },
    )
    await this.snapshotRepository.update(snapshot.id, {
      updateData: {
        state: SnapshotState.BUILD_FAILED,
        errorReason: message,
      },
      entity: snapshot,
    })
  }

  async persistSnapshotFromSandbox(params: PersistSnapshotFromSandboxParams): Promise<Snapshot> {
    return persistSnapshotFromSandbox(
      {
        snapshotRepository: this.snapshotRepository,
        snapshotRunnerRepository: this.snapshotRunnerRepository,
        eventEmitter: this.eventEmitter,
      },
      params,
    )
  }

  async removeSnapshot(snapshotId: string): Promise<void> {
    const snapshot = await this.getSnapshot(snapshotId)
    await this.snapshotRepository.update(snapshot.id, {
      updateData: { state: SnapshotState.REMOVING },
      entity: snapshot,
    })
  }

  async getAllSnapshots(
    page = 1,
    limit = 10,
    filters?: { name?: string },
    sort?: { field?: SnapshotSortField; direction?: SnapshotSortDirection },
  ): Promise<PaginatedList<Snapshot>> {
    const pageNum = Number(page)
    const limitNum = Number(limit)
    const { name } = filters || {}
    const { field: sortField = SnapshotSortField.LAST_USED_AT, direction: sortDirection = SnapshotSortDirection.DESC } =
      sort || {}

    const where: FindOptionsWhere<Snapshot> = {
      ...(name ? { name: ILike(`%${name}%`) } : {}),
      hideFromUsers: false,
    }

    const [items, total] = await this.snapshotRepository.findAndCount({
      where,
      order: {
        [sortField]: {
          direction: sortDirection,
          nulls: 'LAST',
        },
        ...(sortField !== SnapshotSortField.CREATED_AT && { createdAt: 'DESC' }),
      },
      skip: (pageNum - 1) * limitNum,
      take: limitNum,
    })

    return {
      items,
      total,
      page: pageNum,
      totalPages: Math.ceil(total / limitNum),
    }
  }

  async getSnapshot(snapshotIdOrName: string): Promise<Snapshot> {
    const where: FindOptionsWhere<Snapshot>[] = [{ name: snapshotIdOrName }]
    if (isUUID(snapshotIdOrName)) {
      where.push({ id: snapshotIdOrName })
    }

    const snapshot = await this.snapshotRepository.findOne({ where })
    if (!snapshot) {
      throw new NotFoundException(`Snapshot ${snapshotIdOrName} not found`)
    }
    return snapshot
  }

  async getSnapshotByName(snapshotName: string): Promise<Snapshot> {
    return this.getSnapshot(snapshotName)
  }

  async getBuildLogsUrl(snapshot: Snapshot): Promise<string> {
    if (!snapshot.initialRunnerId) {
      throw new NotFoundException(`Snapshot ${snapshot.id} has no initial runner`)
    }

    const runner = await this.runnerService.findOneOrFail(snapshot.initialRunnerId)
    const baseUrl =
      runner.proxyUrl ||
      `${this.configService.getOrThrow('proxy.protocol')}://${this.configService.getOrThrow('proxy.domain')}`

    return `${baseUrl}/snapshots/${snapshot.id}/build-logs`
  }

  async rollbackPendingUsage(_pendingSnapshotCountIncrement?: number): Promise<void> {
    return
  }

  @OnEvent(SandboxEvents.CREATED)
  private async handleSandboxCreatedEvent(event: SandboxCreatedEvent): Promise<void> {
    if (!event.sandbox.snapshot) {
      return
    }

    const snapshot = await this.snapshotRepository.findOne({ where: { name: event.sandbox.snapshot } })
    if (!snapshot) {
      return
    }

    await this.snapshotRepository.update(snapshot.id, { updateData: { lastUsedAt: event.sandbox.createdAt } }, true)
  }

  async activateSnapshot(snapshotId: string): Promise<Snapshot> {
    const snapshot = await this.getSnapshot(snapshotId)
    if (snapshot.state === SnapshotState.ACTIVE) {
      return snapshot
    }

    const updatedSnapshot = await this.snapshotRepository.update(snapshot.id, {
      updateData: { state: SnapshotState.ACTIVE },
      entity: snapshot,
    })
    this.eventEmitter.emit(SnapshotEvents.ACTIVATED, new SnapshotActivatedEvent(updatedSnapshot))
    return updatedSnapshot
  }

  async canCleanupImage(imageName: string): Promise<boolean> {
    const snapshot = await this.snapshotRepository.findOne({
      where: {
        state: Not(In([SnapshotState.ERROR, SnapshotState.BUILD_FAILED])),
        ref: imageName,
      },
    })

    if (snapshot) {
      return false
    }

    const sandbox = await this.sandboxRepository.findOne({
      where: [
        {
          existingBackupSnapshots: Raw((alias) => `${alias} @> '[{"snapshotName":"${imageName}"}]'::jsonb`),
        },
        {
          existingBackupSnapshots: Raw((alias) => `${alias} @> '[{"imageName":"${imageName}"}]'::jsonb`),
        },
        {
          backupSnapshot: imageName,
        },
      ],
    })

    return !(sandbox && sandbox.state !== SandboxState.DESTROYED)
  }

  async deactivateSnapshot(snapshotId: string): Promise<void> {
    const snapshot = await this.getSnapshot(snapshotId)
    if (snapshot.state === SnapshotState.INACTIVE) {
      return
    }

    await this.snapshotRepository.update(snapshot.id, {
      updateData: { state: SnapshotState.INACTIVE },
      entity: snapshot,
    })

    try {
      const countActiveSnapshots = await this.snapshotRepository.count({
        where: {
          state: SnapshotState.ACTIVE,
          ref: snapshot.ref,
        },
      })

      if (countActiveSnapshots === 0) {
        const result = await this.snapshotRunnerRepository.update(
          { snapshotRef: snapshot.ref },
          { state: SnapshotRunnerState.REMOVING },
        )
        this.logger.debug(
          `Deactivated snapshot ${snapshot.id} and marked ${result.affected} SnapshotRunners for removal`,
        )
      }
    } catch (error) {
      this.logger.error(`Deactivated snapshot ${snapshot.id}, but failed to mark snapshot runners for removal`, error)
    }
  }

  getEntrypointFromDockerfile(dockerfileContent: string): string[] {
    const matches = [...dockerfileContent.matchAll(/ENTRYPOINT\s+(.*)/g)]
    const entrypointMatch = matches.length ? matches[matches.length - 1] : null
    if (entrypointMatch) {
      const rawEntrypoint = entrypointMatch[1].trim()
      try {
        const parsed = JSON.parse(rawEntrypoint)
        if (Array.isArray(parsed)) {
          return parsed
        }
      } catch {
        return [rawEntrypoint.replace(/["']/g, '')]
      }
    }
    return ['sleep', 'infinity']
  }

  @OnAsyncEvent({
    event: RunnerEvents.DELETED,
  })
  async handleRunnerDeletedEvent(payload: RunnerDeletedEvent): Promise<void> {
    await payload.entityManager.update(
      SnapshotRunner,
      { runnerId: payload.runnerId },
      { state: SnapshotRunnerState.REMOVING },
    )
  }

  @Cron(CronExpression.EVERY_MINUTE, { name: 'cleanup-failed-snapshot-runners' })
  @LogExecution('cleanup-failed-snapshot-runners')
  @WithInstrumentation()
  async cleanupFailedSnapshotRunners(): Promise<void> {
    const retentionHours = this.configService.getOrThrow('failedSnapshotRunnerRetentionHours')
    const cutoff = new Date()
    cutoff.setHours(cutoff.getHours() - retentionHours)

    const result = await this.snapshotRunnerRepository.delete({
      snapshotRef: Like('daytona-%'),
      state: SnapshotRunnerState.ERROR,
      updatedAt: LessThan(cutoff),
    })

    if (result.affected && result.affected > 0) {
      this.logger.debug(`Cleaned up ${result.affected} failed snapshot runners`)
    }
  }

  @Cron(CronExpression.EVERY_MINUTE, { name: 'cleanup-decommissioned-snapshot-runners' })
  @LogExecution('cleanup-decommissioned-snapshot-runners')
  @WithInstrumentation()
  async cleanupDecommissionedSnapshotRunners(): Promise<void> {
    const cutoff = new Date()
    cutoff.setHours(cutoff.getHours() - 1)

    const snapshotRunners = await this.snapshotRunnerRepository
      .createQueryBuilder('sr')
      .innerJoin('runner', 'r', 'r.id = sr."runnerId"::uuid')
      .where('r.state = :runnerState', { runnerState: RunnerState.DECOMMISSIONED })
      .andWhere('sr."updatedAt" < :cutoff', { cutoff })
      .select('sr.id')
      .take(500)
      .getMany()

    if (snapshotRunners.length === 0) {
      return
    }

    const ids = snapshotRunners.map((sr) => sr.id)
    await this.snapshotRunnerRepository.delete(ids)
    this.logger.debug(`Cleaned up ${ids.length} snapshot runners from decommissioned runners`)
  }
}
