/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { ApiProperty, ApiPropertyOptional, ApiSchema } from '@nestjs/swagger'
import { IsBoolean, IsNumber, IsOptional, IsString } from 'class-validator'
import { TypedConfigService } from '../typed-config.service'

@ApiSchema({ name: 'DaytonaConfiguration' })
export class ConfigurationDto {
  @ApiProperty({
    description: 'Daytona version',
    example: '0.0.1',
  })
  @IsString()
  version: string

  @ApiProperty({
    description: 'Proxy template URL',
    example: 'https://{{PORT}}-{{sandboxId}}.proxy.example.com',
  })
  @IsString()
  proxyTemplateUrl: string

  @ApiProperty({
    description: 'Toolbox template URL',
    example: 'https://proxy.example.com/toolbox',
  })
  @IsString()
  proxyToolboxUrl: string

  @ApiProperty({
    description: 'Default snapshot for sandboxes',
    example: 'ubuntu:22.04',
  })
  @IsString()
  defaultSnapshot: string

  @ApiProperty({
    description: 'Dashboard URL',
    example: 'https://dashboard.example.com',
  })
  @IsString()
  dashboardUrl: string

  @ApiProperty({
    description: 'Maximum auto-archive interval in minutes',
    example: 43200,
  })
  @IsNumber()
  maxAutoArchiveInterval: number

  @ApiProperty({
    description: 'Whether maintenance mode is enabled',
    example: false,
  })
  @IsBoolean()
  maintananceMode: boolean

  @ApiProperty({
    description: 'Current environment',
    example: 'production',
  })
  @IsString()
  environment: string

  @ApiPropertyOptional({
    description: 'SSH Gateway command',
    example: 'ssh -p 2222 {{TOKEN}}@localhost',
  })
  @IsOptional()
  @IsString()
  sshGatewayCommand?: string

  @ApiPropertyOptional({
    description: 'Base64 encoded SSH Gateway public key',
    example: 'ssh-gateway-public-key',
  })
  @IsOptional()
  @IsString()
  sshGatewayPublicKey?: string

  constructor(configService: TypedConfigService) {
    this.version = configService.getOrThrow('version')
    this.proxyTemplateUrl = configService.getOrThrow('proxy.templateUrl')
    this.proxyToolboxUrl = configService.getOrThrow('proxy.toolboxUrl')
    this.defaultSnapshot = configService.getOrThrow('defaultSnapshot')
    this.dashboardUrl = configService.getOrThrow('dashboardUrl')
    this.maxAutoArchiveInterval = configService.getOrThrow('maxAutoArchiveInterval')
    this.maintananceMode = configService.getOrThrow('maintananceMode')
    this.environment = configService.getOrThrow('environment')
    this.sshGatewayCommand = configService.get('sshGateway.command')
    this.sshGatewayPublicKey = configService.get('sshGateway.publicKey')
  }
}
