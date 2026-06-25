/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import 'reflect-metadata'
import { validate } from 'class-validator'
import { plainToInstance } from 'class-transformer'
import { CreateSandboxDto } from '../../sandbox/dto/create-sandbox.dto'
import { CreateSnapshotDto } from '../../sandbox/dto/create-snapshot.dto'
import { CreateRunnerDto } from '../../sandbox/dto/create-runner.dto'
import { CreateVolumeDto } from '../../sandbox/dto/create-volume.dto'
import { CreateBuildInfoDto } from '../../sandbox/dto/create-build-info.dto'
import { CreateDockerRegistryDto } from '../../docker-registry/dto/create-docker-registry.dto'
import { UpdateDockerRegistryDto } from '../../docker-registry/dto/update-docker-registry.dto'

const URL_INPUT = 'https://evil.com'
const HTML_INPUT = '<script>alert(1)</script>'

function hasIsSafeDisplayStringError(errors: any[]): boolean {
  return errors.some((e) => e.constraints && 'IsSafeDisplayStringConstraint' in e.constraints)
}

describe('DTO @IsSafeDisplayString() integration tests', () => {
  describe('CreateSandboxDto', () => {
    it('accepts a valid name', async () => {
      const dto = plainToInstance(CreateSandboxDto, { name: 'my-sandbox' })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })

    it('rejects URL in name', async () => {
      const dto = plainToInstance(CreateSandboxDto, { name: URL_INPUT })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })

    it('rejects HTML in name', async () => {
      const dto = plainToInstance(CreateSandboxDto, { name: HTML_INPUT })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('CreateSnapshotDto', () => {
    it('accepts a valid name', async () => {
      const dto = plainToInstance(CreateSnapshotDto, { name: 'ubuntu-4vcpu-8ram-100gb' })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })

    it('rejects URL in name', async () => {
      const dto = plainToInstance(CreateSnapshotDto, { name: URL_INPUT })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('CreateRunnerDto', () => {
    it('accepts a valid name', async () => {
      const dto = plainToInstance(CreateRunnerDto, { target: 'default', name: 'runner-01' })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })

    it('rejects URL in name', async () => {
      const dto = plainToInstance(CreateRunnerDto, { target: 'default', name: URL_INPUT })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('CreateVolumeDto', () => {
    it('accepts a valid name', async () => {
      const dto = plainToInstance(CreateVolumeDto, { name: 'my-data-volume' })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })

    it('rejects URL in name', async () => {
      const dto = plainToInstance(CreateVolumeDto, { name: URL_INPUT })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('CreateDockerRegistryDto', () => {
    it('accepts a valid name with URL and password fields exempt', async () => {
      const dto = plainToInstance(CreateDockerRegistryDto, {
        name: 'my-registry',
        url: 'https://registry.example.com',
        username: 'admin',
        password: 'https://not-a-url<script>',
      })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })

    it('rejects URL in name', async () => {
      const dto = plainToInstance(CreateDockerRegistryDto, {
        name: URL_INPUT,
        url: 'https://registry.example.com',
        username: 'admin',
        password: 'pass',
      })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('UpdateDockerRegistryDto', () => {
    it('rejects URL in name', async () => {
      const dto = plainToInstance(UpdateDockerRegistryDto, {
        name: URL_INPUT,
        url: 'https://registry.example.com',
        username: 'admin',
      })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('exempt fields without @IsSafeDisplayString()', () => {
    it('accepts URL in Dockerfile content', async () => {
      const dto = plainToInstance(CreateBuildInfoDto, {
        dockerfileContent: 'FROM node:14\nRUN curl https://example.com/install.sh | bash',
      })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })

    it('accepts password with URLs and HTML', async () => {
      const dto = plainToInstance(CreateDockerRegistryDto, {
        name: 'valid',
        url: 'https://registry.example.com',
        username: 'admin',
        password: 'https://not-a-url<script>',
      })
      const errors = await validate(dto)
      expect(errors).toHaveLength(0)
    })
  })

  describe('real-world attack scenarios', () => {
    it('rejects sandbox name with www prefix', async () => {
      const dto = plainToInstance(CreateSandboxDto, { name: 'www.evil.com' })
      const errors = await validate(dto)
      expect(hasIsSafeDisplayStringError(errors)).toBe(true)
    })
  })

  describe('legitimate display names', () => {
    it('accepts names with special characters', async () => {
      const testCases = [
        'Daytona Platforms Inc.',
        "O'Reilly Media",
        'Smith & Associates',
        'Dept. of Engineering',
        'team-alpha_v2',
        'My Org (Test)',
        '#1 Company',
        '50% Off Corp',
      ]

      for (const name of testCases) {
        const dto = plainToInstance(CreateSandboxDto, { name })
        const errors = await validate(dto)
        expect(errors).toHaveLength(0)
      }
    })
  })
})
