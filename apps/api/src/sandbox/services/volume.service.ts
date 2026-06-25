/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ConflictException, Injectable, Logger, NotFoundException, ServiceUnavailableException } from '@nestjs/common'
import { InjectRepository } from '@nestjs/typeorm'
import { Repository, Not, In, FindOptionsWhere } from 'typeorm'
import { Volume } from '../entities/volume.entity'
import { VolumeState } from '../enums/volume-state.enum'
import { CreateVolumeDto } from '../dto/create-volume.dto'
import { v4 as uuidv4 } from 'uuid'
import { BadRequestError } from '../../exceptions/bad-request.exception'
import { isValidUuid } from '../../common/utils/uuid'
import { OnEvent } from '@nestjs/event-emitter'
import { SandboxEvents } from '../constants/sandbox-events.constants'
import { SandboxCreatedEvent } from '../events/sandbox-create.event'
import { TypedConfigService } from '../../config/typed-config.service'
import { RedisLockProvider } from '../common/redis-lock.provider'
import { SandboxRepository } from '../repositories/sandbox.repository'
import { SandboxDesiredState } from '../enums/sandbox-desired-state.enum'

@Injectable()
export class VolumeService {
  private readonly logger = new Logger(VolumeService.name)

  constructor(
    @InjectRepository(Volume)
    private readonly volumeRepository: Repository<Volume>,
    private readonly sandboxRepository: SandboxRepository,
    private readonly configService: TypedConfigService,
    private readonly redisLockProvider: RedisLockProvider,
  ) {}

  async create(createVolumeDto: CreateVolumeDto): Promise<Volume> {
    if (!this.configService.get('s3.endpoint')) {
      throw new ServiceUnavailableException('Object storage is not configured')
    }

    const volume = new Volume()
    volume.id = uuidv4()
    volume.name = createVolumeDto.name || volume.id

    const existingVolume = await this.volumeRepository.findOne({
      where: {
        name: volume.name,
        state: Not(VolumeState.DELETED),
      },
    })

    if (existingVolume) {
      throw new BadRequestError(`Volume with name ${volume.name} already exists`)
    }

    volume.state = VolumeState.PENDING_CREATE

    const savedVolume = await this.volumeRepository.save(volume)
    this.logger.debug(`Created volume ${savedVolume.id}`)
    return savedVolume
  }

  async delete(volumeId: string): Promise<void> {
    const volume = await this.volumeRepository.findOne({
      where: {
        id: volumeId,
      },
    })

    if (!volume) {
      throw new NotFoundException(`Volume with ID ${volumeId} not found`)
    }

    if (volume.state !== VolumeState.READY && volume.state !== VolumeState.ERROR) {
      throw new BadRequestError(
        `Volume must be in '${VolumeState.READY}' or '${VolumeState.ERROR}' state in order to be deleted`,
      )
    }

    // Check if any non-destroyed sandboxes are using this volume
    const sandboxUsingVolume = await this.sandboxRepository
      .createQueryBuilder('sandbox')
      .where('sandbox.volumes @> :volFilter::jsonb', {
        volFilter: JSON.stringify([{ volumeId }]),
      })
      .andWhere('sandbox.desiredState != :destroyed', {
        destroyed: SandboxDesiredState.DESTROYED,
      })
      .select(['sandbox.id', 'sandbox.name'])
      .getOne()

    if (sandboxUsingVolume) {
      throw new ConflictException(
        `Volume cannot be deleted because it is in use by one or more sandboxes (e.g. ${sandboxUsingVolume.name})`,
      )
    }

    // Update state to mark as deleting
    volume.state = VolumeState.PENDING_DELETE
    await this.volumeRepository.save(volume)
    this.logger.debug(`Marked volume ${volumeId} for deletion`)
  }

  async findOne(volumeId: string): Promise<Volume> {
    const volume = await this.volumeRepository.findOne({
      where: { id: volumeId },
    })

    if (!volume) {
      throw new NotFoundException(`Volume with ID ${volumeId} not found`)
    }

    return volume
  }

  async findAll(includeDeleted = false): Promise<Volume[]> {
    return this.volumeRepository.find({
      where: {
        ...(includeDeleted ? {} : { state: Not(VolumeState.DELETED) }),
      },
      order: {
        lastUsedAt: {
          direction: 'DESC',
          nulls: 'LAST',
        },
        createdAt: 'DESC',
      },
    })
  }

  async findByName(name: string): Promise<Volume> {
    const volume = await this.volumeRepository.findOne({
      where: {
        name,
        state: Not(VolumeState.DELETED),
      },
    })

    if (!volume) {
      throw new NotFoundException(`Volume with name ${name} not found`)
    }

    return volume
  }

  // Looks up volumes where each reference may be a volume ID or a volume name, and
  // returns them keyed by the requested reference. Throws when a reference is unknown,
  // ambiguous, or points at a volume that is not ready.
  async getVolumesByIdOrName(volumeIdOrNames: string[]): Promise<Map<string, Volume>> {
    if (!volumeIdOrNames.length) {
      return new Map()
    }

    // The id column is a Postgres uuid — filtering it by a non-UUID string makes the
    // query itself throw, so only UUID-shaped references go into the id filter. Names
    // may also be UUID-shaped, so every reference goes into the name filter.
    // Postgres compares uuids case-insensitively but the in-memory maps below cannot,
    // so UUID-shaped references are canonicalized to lowercase here and when matching.
    const uuidRefs = volumeIdOrNames
      .filter((idOrName) => isValidUuid(idOrName))
      .map((idOrName) => idOrName.toLowerCase())
    const where: FindOptionsWhere<Volume>[] = [{ name: In(volumeIdOrNames), state: Not(VolumeState.DELETED) }]
    if (uuidRefs.length > 0) {
      where.push({ id: In(uuidRefs), state: Not(VolumeState.DELETED) })
    }

    const foundVolumes = await this.volumeRepository.find({ where })

    const volumesById = new Map<string, Volume>()
    const volumesByName = new Map<string, Volume>()
    for (const foundVolume of foundVolumes) {
      volumesById.set(foundVolume.id, foundVolume)
      volumesByName.set(foundVolume.name, foundVolume)
    }

    const volumes = new Map<string, Volume>()
    for (const idOrName of volumeIdOrNames) {
      let matchedById: Volume | undefined
      if (isValidUuid(idOrName)) {
        matchedById = volumesById.get(idOrName.toLowerCase())
      }
      const matchedByName = volumesByName.get(idOrName)
      if (matchedById !== undefined && matchedByName !== undefined && matchedById.id !== matchedByName.id) {
        throw new BadRequestError(
          `Volume reference '${idOrName}' matches one volume's ID and another volume's name; rename the volume to remove the ambiguity`,
        )
      }

      let matchedVolume: Volume | undefined
      if (matchedById !== undefined) {
        matchedVolume = matchedById
      } else {
        matchedVolume = matchedByName
      }

      if (matchedVolume === undefined) {
        throw new NotFoundException(`Volume '${idOrName}' not found`)
      }
      if (matchedVolume.state !== VolumeState.READY) {
        throw new BadRequestError(
          `Volume '${matchedVolume.name}' is not in a ready state. Current state: ${matchedVolume.state}`,
        )
      }

      volumes.set(idOrName, matchedVolume)
    }

    return volumes
  }

  @OnEvent(SandboxEvents.CREATED)
  private async handleSandboxCreatedEvent(event: SandboxCreatedEvent) {
    if (!event.sandbox.volumes.length) {
      return
    }

    try {
      const volumeIds = event.sandbox.volumes.map((vol) => vol.volumeId)
      const volumes = await this.volumeRepository.find({ where: { id: In(volumeIds) } })

      const results = await Promise.allSettled(
        volumes.map(async (volume) => {
          // Update once per minute at most
          if (!(await this.redisLockProvider.lock(`volume:${volume.id}:update-last-used`, 60))) {
            return
          }
          volume.lastUsedAt = event.sandbox.createdAt
          return this.volumeRepository.save(volume)
        }),
      )

      results.forEach((result) => {
        if (result.status === 'rejected') {
          this.logger.error(
            `Failed to update volume lastUsedAt timestamp for sandbox ${event.sandbox.id}: ${result.reason}`,
          )
        }
      })
    } catch (err) {
      this.logger.error(err)
    }
  }
}
