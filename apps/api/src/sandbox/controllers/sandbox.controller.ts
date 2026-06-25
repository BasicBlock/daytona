/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import {
  Body,
  Controller,
  Delete,
  Get,
  HttpCode,
  Logger,
  Next,
  NotFoundException,
  Param,
  ParseBoolPipe,
  Post,
  Put,
  Query,
  RawBodyRequest,
  Request,
  Res,
} from '@nestjs/common'
import { ApiOperation, ApiParam, ApiQuery, ApiResponse, ApiTags } from '@nestjs/swagger'
import { IncomingMessage, ServerResponse } from 'http'
import { NextFunction } from 'http-proxy-middleware/dist/types'
import { BadRequestError } from '../../exceptions/bad-request.exception'
import { UrlDto } from '../../common/dto/url.dto'
import { CreateSandboxDto } from '../dto/create-sandbox.dto'
import { CreateSandboxSnapshotDto } from '../dto/create-sandbox-snapshot.dto'
import { ForkSandboxDto } from '../dto/fork-sandbox.dto'
import { ListSandboxesQueryDto } from '../dto/list-sandboxes-query.dto'
import {
  DEFAULT_SANDBOX_SORT_DIRECTION_DEPRECATED,
  DEFAULT_SANDBOX_SORT_FIELD_DEPRECATED,
  ListSandboxesQueryDtoDeprecated,
} from '../dto/list-sandboxes-query.deprecated.dto'
import { ListSandboxesResponseDto } from '../dto/list-sandboxes-response.dto'
import { PaginatedSandboxesDtoDeprecated } from '../dto/paginated-sandboxes.deprecated.dto'
import { PortPreviewUrlDto, SignedPortPreviewUrlDto } from '../dto/port-preview-url.dto'
import { RegistryPushAccessDto } from '../dto/registry-push-access-dto'
import { ResizeSandboxDto } from '../dto/resize-sandbox.dto'
import { SandboxDto, SandboxLabelsDto } from '../dto/sandbox.dto'
import { SshAccessDto, SshAccessValidationDto } from '../dto/ssh-access.dto'
import { ToolboxProxyUrlDto } from '../dto/toolbox-proxy-url.dto'
import { UpdateSandboxNetworkSettingsDto } from '../dto/update-sandbox-network-settings.dto'
import { UpdateSandboxStateDto } from '../dto/update-sandbox-state.dto'
import { Sandbox } from '../entities/sandbox.entity'
import { SandboxState } from '../enums/sandbox-state.enum'
import { LogProxy } from '../proxy/log-proxy'
import { RunnerService } from '../services/runner.service'
import { SandboxService } from '../services/sandbox.service'

@Controller('sandbox')
@ApiTags('sandbox')
export class SandboxController {
  private readonly logger = new Logger(SandboxController.name)

  constructor(
    private readonly runnerService: RunnerService,
    private readonly sandboxService: SandboxService,
  ) {}

  @Get()
  @ApiOperation({ summary: 'List sandboxes', operationId: 'listSandboxes' })
  @ApiResponse({ status: 200, type: ListSandboxesResponseDto })
  async listSandboxes(@Query() query: ListSandboxesQueryDto): Promise<ListSandboxesResponseDto> {
    return this.sandboxService.search(query)
  }

  @Get('paginated')
  @ApiOperation({
    summary: '[DEPRECATED] List all sandboxes paginated',
    operationId: 'listSandboxesPaginated_deprecated',
    deprecated: true,
  })
  @ApiResponse({ status: 200, type: PaginatedSandboxesDtoDeprecated })
  async listSandboxesPaginated(
    @Query() queryParams: ListSandboxesQueryDtoDeprecated,
  ): Promise<PaginatedSandboxesDtoDeprecated> {
    const {
      page,
      limit,
      id,
      name,
      labels,
      includeErroredDeleted: includeErroredDestroyed,
      states,
      snapshots,
      targets,
      minCpu,
      maxCpu,
      minMemoryGiB,
      maxMemoryGiB,
      minDiskGiB,
      maxDiskGiB,
      lastEventAfter,
      lastEventBefore,
      sort: sortField = DEFAULT_SANDBOX_SORT_FIELD_DEPRECATED,
      order: sortDirection = DEFAULT_SANDBOX_SORT_DIRECTION_DEPRECATED,
    } = queryParams

    let parsedLabels: { [key: string]: string } | undefined
    if (labels) {
      try {
        parsedLabels = JSON.parse(labels)
      } catch {
        throw new BadRequestError('Invalid labels JSON format')
      }
    }

    const result = await this.sandboxService.findAllPaginatedDeprecated(
      page,
      limit,
      {
        id,
        name,
        labels: parsedLabels,
        includeErroredDestroyed,
        states,
        snapshots,
        targets,
        minCpu,
        maxCpu,
        minMemoryGiB,
        maxMemoryGiB,
        minDiskGiB,
        maxDiskGiB,
        lastEventAfter,
        lastEventBefore,
      },
      {
        field: sortField,
        direction: sortDirection,
      },
    )

    return {
      items: await this.sandboxService.toSandboxDtos(result.items),
      total: result.total,
      page: result.page,
      totalPages: result.totalPages,
    }
  }

  @Post()
  @HttpCode(200)
  @ApiOperation({ summary: 'Create a new sandbox', operationId: 'createSandbox' })
  @ApiResponse({ status: 200, type: SandboxDto })
  async createSandbox(@Body() createSandboxDto: CreateSandboxDto): Promise<SandboxDto> {
    if (createSandboxDto.buildInfo) {
      if (createSandboxDto.snapshot) {
        throw new BadRequestError('Cannot specify a snapshot when using a build info entry')
      }
      if (createSandboxDto.linkedSandbox) {
        throw new BadRequestError(
          'linkedSandbox is not supported with declarative builds. Create a sandbox from a snapshot',
        )
      }
      return this.sandboxService.createFromBuildInfo(createSandboxDto)
    }

    if (createSandboxDto.cpu || createSandboxDto.gpu || createSandboxDto.memory || createSandboxDto.disk) {
      throw new BadRequestError('Cannot specify Sandbox resources when using a snapshot')
    }
    if (createSandboxDto.gpuType && createSandboxDto.gpuType.length > 0) {
      throw new BadRequestError(
        'Cannot specify GPU type when creating sandbox from snapshot. GPU type is inherited from snapshot.',
      )
    }
    return this.sandboxService.createFromSnapshot(createSandboxDto)
  }

  @Get('for-runner')
  @ApiOperation({ summary: 'Get sandboxes for a runner', operationId: 'getSandboxesForRunner' })
  @ApiQuery({ name: 'runnerId', required: true, type: String })
  @ApiQuery({ name: 'states', required: false, type: String })
  @ApiQuery({ name: 'skipReconcilingSandboxes', required: false, type: Boolean })
  @ApiResponse({ status: 200, type: [SandboxDto] })
  async getSandboxesForRunner(
    @Query('runnerId') runnerId: string,
    @Query('states') states?: string,
    @Query('skipReconcilingSandboxes') skipReconcilingSandboxes?: string,
  ): Promise<SandboxDto[]> {
    if (!runnerId) {
      throw new BadRequestError('runnerId is required')
    }

    const stateArray = states
      ? states.split(',').map((s) => {
          if (!Object.values(SandboxState).includes(s as SandboxState)) {
            throw new BadRequestError(`Invalid sandbox state: ${s}`)
          }
          return s as SandboxState
        })
      : undefined

    const sandboxes = await this.sandboxService.findByRunnerId(
      runnerId,
      stateArray,
      skipReconcilingSandboxes === 'true',
    )

    return this.sandboxService.toSandboxDtos(sandboxes)
  }

  @Get(':sandboxIdOrName')
  @ApiOperation({ summary: 'Get sandbox details', operationId: 'getSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async getSandbox(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.findOneByIdOrName(sandboxIdOrName)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Delete(':sandboxIdOrName')
  @ApiOperation({ summary: 'Delete sandbox', operationId: 'deleteSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async deleteSandbox(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.destroy(sandboxIdOrName)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/recover')
  @HttpCode(200)
  @ApiOperation({ summary: 'Recover sandbox from error state', operationId: 'recoverSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiQuery({ name: 'skipStart', required: false, type: Boolean })
  @ApiResponse({ status: 200, type: SandboxDto })
  async recoverSandbox(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Query('skipStart', new ParseBoolPipe({ optional: true })) skipStart?: boolean,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.recover(sandboxIdOrName, skipStart)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/start')
  @HttpCode(200)
  @ApiOperation({ summary: 'Start or resume sandbox', operationId: 'startSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async startSandbox(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.start(sandboxIdOrName)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/stop')
  @HttpCode(200)
  @ApiOperation({ summary: 'Stop sandbox', operationId: 'stopSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiQuery({ name: 'force', required: false, type: Boolean })
  @ApiResponse({ status: 200, type: SandboxDto })
  async stopSandbox(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Query('force', new ParseBoolPipe({ optional: true })) force?: boolean,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.stop(sandboxIdOrName, force)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/pause')
  @HttpCode(200)
  @ApiOperation({ summary: 'Pause sandbox', operationId: 'pauseSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async pauseSandbox(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.pause(sandboxIdOrName)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/resize')
  @HttpCode(200)
  @ApiOperation({ summary: 'Resize sandbox resources', operationId: 'resizeSandbox' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async resizeSandbox(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Body() resizeSandboxDto: ResizeSandboxDto,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.resize(sandboxIdOrName, resizeSandboxDto)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Put(':sandboxIdOrName/labels')
  @ApiOperation({ summary: 'Replace sandbox labels', operationId: 'replaceLabels' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxLabelsDto })
  async replaceLabels(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Body() labelsDto: SandboxLabelsDto,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.replaceLabels(sandboxIdOrName, labelsDto.labels)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Put(':sandboxId/state')
  @ApiOperation({ summary: 'Update sandbox state', operationId: 'updateSandboxState' })
  @ApiParam({ name: 'sandboxId', type: String })
  @ApiResponse({ status: 200 })
  async updateSandboxState(
    @Param('sandboxId') sandboxId: string,
    @Body() updateStateDto: UpdateSandboxStateDto,
  ): Promise<void> {
    await this.sandboxService.updateState(
      sandboxId,
      updateStateDto.state,
      updateStateDto.recoverable,
      updateStateDto.errorReason,
    )
  }

  @Post(':sandboxIdOrName/backup')
  @ApiOperation({ summary: 'Create sandbox backup', operationId: 'createBackup' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async createBackup(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.createBackup(sandboxIdOrName)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/snapshot')
  @HttpCode(200)
  @ApiOperation({ summary: 'Create a snapshot from a sandbox', operationId: 'createSandboxSnapshot' })
  @ApiResponse({ status: 200, type: SandboxDto })
  async createSandboxSnapshot(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Body() dto: CreateSandboxSnapshotDto,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.createSnapshotFromSandbox(sandboxIdOrName, dto)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/fork')
  @HttpCode(200)
  @ApiOperation({ summary: 'Fork a sandbox', operationId: 'forkSandbox' })
  @ApiResponse({ status: 200, type: SandboxDto })
  async forkSandbox(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Body() dto: ForkSandboxDto,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.forkSandbox(sandboxIdOrName, dto)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Get(':sandboxIdOrName/forks')
  @ApiOperation({ summary: 'Get sandbox fork children', operationId: 'getSandboxForks' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiQuery({ name: 'includeDestroyed', required: false, type: Boolean })
  @ApiResponse({ status: 200, type: [SandboxDto] })
  async getSandboxForks(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Query('includeDestroyed') includeDestroyed?: boolean,
  ): Promise<SandboxDto[]> {
    const children = await this.sandboxService.getForkChildren(sandboxIdOrName, includeDestroyed)
    return this.sandboxService.toSandboxDtos(children)
  }

  @Get(':sandboxIdOrName/parent')
  @ApiOperation({ summary: 'Get sandbox fork parent', operationId: 'getSandboxParent' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async getSandboxParent(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const parent = await this.sandboxService.getForkParent(sandboxIdOrName)
    if (!parent) {
      throw new NotFoundException(`Parent sandbox not found for sandbox ${sandboxIdOrName}`)
    }
    return this.sandboxService.toSandboxDto(parent)
  }

  @Get(':sandboxIdOrName/ancestors')
  @ApiOperation({ summary: 'Get sandbox fork ancestor chain', operationId: 'getSandboxAncestors' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: [SandboxDto] })
  async getSandboxAncestors(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto[]> {
    const ancestors = await this.sandboxService.getForkAncestors(sandboxIdOrName)
    return this.sandboxService.toSandboxDtos(ancestors)
  }

  @Post(':sandboxId/last-activity')
  @ApiOperation({ summary: 'Update sandbox last activity', operationId: 'updateLastActivity' })
  @ApiParam({ name: 'sandboxId', type: String })
  @ApiResponse({ status: 201 })
  async updateLastActivity(@Param('sandboxId') sandboxId: string): Promise<void> {
    await this.sandboxService.updateLastActivityAt(sandboxId, new Date())
  }

  @Post(':sandboxIdOrName/autostop/:interval')
  @ApiOperation({ summary: 'Set sandbox auto-stop interval', operationId: 'setAutostopInterval' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiParam({ name: 'interval', type: Number })
  @ApiResponse({ status: 200, type: SandboxDto })
  async setAutostopInterval(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Param('interval') interval: number,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.setAutostopInterval(sandboxIdOrName, interval)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/autoarchive/:interval')
  @ApiOperation({ summary: 'Set sandbox auto-archive interval', operationId: 'setAutoArchiveInterval' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiParam({ name: 'interval', type: Number })
  @ApiResponse({ status: 200, type: SandboxDto })
  async setAutoArchiveInterval(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Param('interval') interval: number,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.setAutoArchiveInterval(sandboxIdOrName, interval)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/autodelete/:interval')
  @ApiOperation({ summary: 'Set sandbox auto-delete interval', operationId: 'setAutoDeleteInterval' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiParam({ name: 'interval', type: Number })
  @ApiResponse({ status: 200, type: SandboxDto })
  async setAutoDeleteInterval(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Param('interval') interval: number,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.setAutoDeleteInterval(sandboxIdOrName, interval)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/network-settings')
  @HttpCode(200)
  @ApiOperation({ summary: 'Update sandbox network settings', operationId: 'updateNetworkSettings' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async updateNetworkSettings(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Body() networkSettings: UpdateSandboxNetworkSettingsDto,
  ): Promise<SandboxDto> {
    if (
      networkSettings.networkBlockAll === undefined &&
      networkSettings.networkAllowList === undefined &&
      networkSettings.domainAllowList === undefined
    ) {
      throw new BadRequestError('At least one of networkBlockAll, networkAllowList or domainAllowList must be provided')
    }
    const sandbox = await this.sandboxService.updateNetworkSettings(
      sandboxIdOrName,
      networkSettings.networkBlockAll,
      networkSettings.networkAllowList,
      networkSettings.domainAllowList,
    )
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Post(':sandboxIdOrName/archive')
  @HttpCode(200)
  @ApiOperation({ summary: 'Archive sandbox', operationId: 'archiveSandbox' })
  @ApiResponse({ status: 200, type: SandboxDto })
  async archiveSandbox(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.archive(sandboxIdOrName)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Get(':sandboxIdOrName/ports/:port/preview-url')
  @ApiOperation({ summary: 'Get preview URL for a sandbox port', operationId: 'getPortPreviewUrl' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiParam({ name: 'port', type: Number })
  @ApiResponse({ status: 200, type: PortPreviewUrlDto })
  async getPortPreviewUrl(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Param('port') port: number,
  ): Promise<PortPreviewUrlDto> {
    return this.sandboxService.getPortPreviewUrl(sandboxIdOrName, port)
  }

  @Get(':sandboxIdOrName/ports/:port/signed-preview-url')
  @ApiOperation({ summary: 'Get signed preview URL for a sandbox port', operationId: 'getSignedPortPreviewUrl' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiParam({ name: 'port', type: Number })
  @ApiQuery({ name: 'expiresInSeconds', required: false, type: Number })
  @ApiResponse({ status: 200, type: SignedPortPreviewUrlDto })
  async getSignedPortPreviewUrl(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Param('port') port: number,
    @Query('expiresInSeconds') expiresInSeconds?: number,
  ): Promise<SignedPortPreviewUrlDto> {
    return this.sandboxService.getSignedPortPreviewUrl(sandboxIdOrName, port, expiresInSeconds)
  }

  @Post(':sandboxIdOrName/ports/:port/signed-preview-url/:token/expire')
  @ApiOperation({ summary: 'Expire signed preview URL for a sandbox port', operationId: 'expireSignedPortPreviewUrl' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiParam({ name: 'port', type: Number })
  @ApiParam({ name: 'token', type: String })
  @ApiResponse({ status: 200 })
  async expireSignedPortPreviewUrl(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Param('port') port: number,
    @Param('token') token: string,
  ): Promise<void> {
    await this.sandboxService.expireSignedPreviewUrlToken(sandboxIdOrName, token, port)
  }

  @Get(':sandboxIdOrName/build-logs')
  @ApiOperation({ summary: 'Get build logs', operationId: 'getBuildLogs', deprecated: true })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiQuery({ name: 'follow', required: false, type: Boolean })
  @ApiResponse({ status: 200 })
  async getBuildLogs(
    @Request() req: RawBodyRequest<IncomingMessage>,
    @Res() res: ServerResponse<IncomingMessage>,
    @Next() next: NextFunction,
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Query('follow', new ParseBoolPipe({ optional: true })) follow?: boolean,
  ): Promise<void> {
    const sandbox = await this.sandboxService.findOneByIdOrName(sandboxIdOrName)
    if (!sandbox.runnerId) {
      throw new NotFoundException(`Sandbox with ID or name ${sandboxIdOrName} has no runner assigned`)
    }
    if (!sandbox.buildInfo) {
      throw new NotFoundException(`Sandbox with ID or name ${sandboxIdOrName} has no build info`)
    }

    const runner = await this.runnerService.findOneOrFail(sandbox.runnerId)
    if (!runner.apiUrl) {
      throw new NotFoundException(`Runner for sandbox ${sandboxIdOrName} has no API URL`)
    }

    return new LogProxy(
      runner.apiUrl,
      sandbox.buildInfo.snapshotRef.split(':')[0],
      follow === true,
      req,
      res,
      next,
    ).create()
  }

  @Get(':sandboxIdOrName/build-logs-url')
  @ApiOperation({ summary: 'Get build logs URL', operationId: 'getBuildLogsUrl' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiResponse({ status: 200, type: UrlDto })
  async getBuildLogsUrl(@Param('sandboxIdOrName') sandboxIdOrName: string): Promise<UrlDto> {
    return new UrlDto(await this.sandboxService.getBuildLogsUrl(sandboxIdOrName))
  }

  @Post(':sandboxIdOrName/ssh-access')
  @HttpCode(200)
  @ApiOperation({ summary: 'Create SSH access for sandbox', operationId: 'createSshAccess' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiQuery({ name: 'expiresInMinutes', required: false, type: Number })
  @ApiResponse({ status: 200, type: SshAccessDto })
  async createSshAccess(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Query('expiresInMinutes') expiresInMinutes?: number,
  ): Promise<SshAccessDto> {
    return this.sandboxService.createSshAccess(sandboxIdOrName, expiresInMinutes)
  }

  @Delete(':sandboxIdOrName/ssh-access')
  @HttpCode(200)
  @ApiOperation({ summary: 'Revoke SSH access for sandbox', operationId: 'revokeSshAccess' })
  @ApiParam({ name: 'sandboxIdOrName', type: String })
  @ApiQuery({ name: 'token', required: false, type: String })
  @ApiResponse({ status: 200, type: SandboxDto })
  async revokeSshAccess(
    @Param('sandboxIdOrName') sandboxIdOrName: string,
    @Query('token') token?: string,
  ): Promise<SandboxDto> {
    const sandbox = await this.sandboxService.revokeSshAccess(sandboxIdOrName, token)
    return this.sandboxService.toSandboxDto(sandbox)
  }

  @Get('ssh-access/validate')
  @ApiOperation({ summary: 'Validate SSH access for sandbox', operationId: 'validateSshAccess' })
  @ApiQuery({ name: 'token', required: true, type: String })
  @ApiResponse({ status: 200, type: SshAccessValidationDto })
  async validateSshAccess(@Query('token') token: string): Promise<SshAccessValidationDto> {
    const result = await this.sandboxService.validateSshAccess(token)
    return SshAccessValidationDto.fromValidationResult(result.valid, result.sandboxId)
  }

  @Get(':sandboxId/toolbox-proxy-url')
  @ApiOperation({ summary: 'Get toolbox proxy URL for a sandbox', operationId: 'getToolboxProxyUrl' })
  @ApiParam({ name: 'sandboxId', type: String })
  @ApiResponse({ status: 200, type: ToolboxProxyUrlDto })
  async getToolboxProxyUrl(@Param('sandboxId') sandboxId: string): Promise<ToolboxProxyUrlDto> {
    return new ToolboxProxyUrlDto(await this.sandboxService.getToolboxProxyUrl(sandboxId))
  }
}
