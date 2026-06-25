/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import type { AxiosInstance, AxiosRequestConfig } from 'axios'
import axios from 'axios'

export class Configuration {
  basePath?: string
  accessToken?: string

  constructor(config: { basePath?: string; accessToken?: string } = {}) {
    this.basePath = config.basePath
    this.accessToken = config.accessToken
  }
}

export type ApiResponse<T> = Promise<{ data: T }>

export type DaytonaConfiguration = {
  version: string
  proxyTemplateUrl: string
  proxyToolboxUrl: string
  defaultSnapshot: string
  dashboardUrl: string
  maxAutoArchiveInterval: number
  maintananceMode: boolean
  environment: string
  sshGatewayCommand?: string
  sshGatewayPublicKey?: string
}

export enum SandboxState {
  UNKNOWN_DEFAULT_OPEN_API = 'unknown_default_open_api',
  CREATING = 'creating',
  RESTORING = 'restoring',
  DESTROYED = 'destroyed',
  DESTROYING = 'destroying',
  STARTED = 'started',
  STOPPED = 'stopped',
  STARTING = 'starting',
  STOPPING = 'stopping',
  ERROR = 'error',
  BUILD_FAILED = 'build_failed',
  PENDING_BUILD = 'pending_build',
  BUILDING_SNAPSHOT = 'building_snapshot',
  UNKNOWN = 'unknown',
  PULLING_SNAPSHOT = 'pulling_snapshot',
  ARCHIVED = 'archived',
  ARCHIVING = 'archiving',
  RESIZING = 'resizing',
  SNAPSHOTTING = 'snapshotting',
  FORKING = 'forking',
  PAUSING = 'pausing',
  PAUSED = 'paused',
  RESUMING = 'resuming',
}

export enum SandboxDesiredState {
  DESTROYED = 'destroyed',
  STARTED = 'started',
  STOPPED = 'stopped',
  RESIZED = 'resized',
  ARCHIVED = 'archived',
  PAUSED = 'paused',
}

export enum SandboxClass {
  UNKNOWN_DEFAULT_OPEN_API = 'unknown_default_open_api',
  LINUX_VM = 'linux-vm',
  CONTAINER = 'container',
  ANDROID = 'android',
  WINDOWS = 'windows',
}

export enum SandboxListSortField {
  NAME = 'name',
  CPU = 'cpu',
  MEMORY_GIB = 'memoryGib',
  DISK_GIB = 'diskGib',
  LAST_ACTIVITY_AT = 'lastActivityAt',
  CREATED_AT = 'createdAt',
}

export enum SandboxListSortDirection {
  ASC = 'asc',
  DESC = 'desc',
}

export enum SnapshotState {
  BUILDING = 'building',
  PENDING = 'pending',
  PULLING = 'pulling',
  ACTIVE = 'active',
  INACTIVE = 'inactive',
  ERROR = 'error',
  BUILD_FAILED = 'build_failed',
  REMOVING = 'removing',
}

export enum GetAllSnapshotsSortEnum {
  NAME = 'name',
  STATE = 'state',
  LAST_USED_AT = 'lastUsedAt',
  CREATED_AT = 'createdAt',
}

export enum GetAllSnapshotsOrderEnum {
  ASC = 'asc',
  DESC = 'desc',
}

export enum VolumeState {
  CREATING = 'creating',
  READY = 'ready',
  PENDING_CREATE = 'pending_create',
  PENDING_DELETE = 'pending_delete',
  DELETING = 'deleting',
  DELETED = 'deleted',
  ERROR = 'error',
}

export enum RunnerState {
  INITIALIZING = 'initializing',
  READY = 'ready',
  DISABLED = 'disabled',
  DECOMMISSIONED = 'decommissioned',
  UNRESPONSIVE = 'unresponsive',
}

export enum GpuType {
  H100 = 'H100',
  RTX_PRO_6000 = 'RTX-PRO-6000',
  UNKNOWN_DEFAULT_OPEN_API = 'unknown_default_open_api',
}

export enum RegistryType {
  INTERNAL = 'internal',
  CUSTOM = 'custom',
  TRANSIENT = 'transient',
  BACKUP = 'backup',
}

export enum TargetType {
  SHARED = 'shared',
  CUSTOM = 'custom',
}

export type BuildInfo = {
  dockerfileContent?: string
  contextHashes?: string[]
  snapshotRef?: string
  createdAt?: Date
  updatedAt?: Date
}

export type SandboxVolume = {
  volumeId?: string
  volumeName?: string
  mountPath: string
}

export type CreateSandbox = {
  name?: string
  snapshot?: string
  image?: string
  imageName?: string
  osUser?: string
  env?: Record<string, string>
  labels?: Record<string, string>
  target?: string
  cpu?: number
  gpu?: number
  gpuType?: GpuType[]
  memory?: number
  disk?: number
  autoStopInterval?: number
  autoArchiveInterval?: number
  autoDeleteInterval?: number
  volumes?: SandboxVolume[]
  buildInfo?: BuildInfo
  [key: string]: unknown
}

export type SandboxListItem = {
  id: string
  name: string
  target: string
  runnerId?: string
  sandboxClass?: SandboxClass
  state: SandboxState
  desiredState?: SandboxDesiredState
  snapshot?: string
  osUser?: string
  errorReason?: string
  recoverable?: boolean
  cpu: number
  gpu: number
  gpuType?: GpuType
  memory: number
  disk: number
  labels: Record<string, string>
  autoStopInterval?: number
  autoArchiveInterval?: number
  autoDeleteInterval?: number
  createdAt?: string
  updatedAt?: string
  lastActivityAt?: string
  daemonVersion?: string
  toolboxProxyUrl?: string
  buildInfo?: BuildInfo
  volumes?: SandboxVolume[]
  networkBlockAll?: boolean
  networkAllowList?: string
  domainAllowList?: string
  [key: string]: unknown
}

export type Sandbox = SandboxListItem

export type ListSandboxesResponse = {
  items: SandboxListItem[]
  nextCursor: string | null
}

export type CreateSnapshot = {
  name: string
  imageName?: string
  entrypoint?: string[]
  cpu?: number
  gpu?: number
  gpuType?: GpuType[]
  memory?: number
  disk?: number
  buildInfo?: BuildInfo
  sandboxClass?: SandboxClass
  target?: string
}

export type SnapshotDto = {
  id: string
  name: string
  imageName?: string
  state: SnapshotState
  size: number | null
  entrypoint: string[] | null
  cpu: number
  gpu: number
  gpuType?: GpuType
  mem: number
  disk: number
  errorReason?: string
  createdAt: Date
  updatedAt: Date
  lastUsedAt: Date | null
  buildInfo?: BuildInfo
  initialRunnerId?: string
  ref?: string
  sandboxClass?: SandboxClass
  targets?: string[]
  general?: boolean
  [key: string]: unknown
}

export type PaginatedSnapshots = {
  items: SnapshotDto[]
  total: number
  page: number
  totalPages: number
}

export type CreateVolume = {
  name: string
}

export type VolumeDto = {
  id: string
  name: string
  state: VolumeState
  createdAt: string
  updatedAt: string
  lastUsedAt?: string
  errorReason?: string
}

export type CreateRunner = {
  target?: string
  name: string
  tags?: string[]
}

export type CreateRunnerResponse = {
  id: string
}

export type Runner = {
  id: string
  domain?: string
  apiUrl?: string
  proxyUrl?: string
  cpu: number
  memory: number
  disk: number
  gpu?: number
  gpuType?: string
  sandboxClass?: SandboxClass
  currentCpuUsagePercentage?: number
  currentMemoryUsagePercentage?: number
  currentDiskUsagePercentage?: number
  currentAllocatedCpu?: number
  currentAllocatedMemoryGiB?: number
  currentAllocatedDiskGiB?: number
  currentSnapshotCount?: number
  currentStartedSandboxes?: number
  availabilityScore?: number
  target: string
  name: string
  state: RunnerState
  lastChecked?: string
  unschedulable: boolean
  tags: string[]
  createdAt: string
  updatedAt: string
  version?: string
  appVersion?: string
  apiVersion?: string
  runnerClass?: string
  [key: string]: unknown
}

export type CreateDockerRegistry = {
  name: string
  url: string
  username: string
  password: string
  project?: string
}

export type UpdateDockerRegistry = {
  name: string
  url: string
  username: string
  password?: string
  project?: string
}

export type DockerRegistry = {
  id: string
  name: string
  url: string
  username: string
  project?: string
  registryType: RegistryType
  createdAt: string
  updatedAt: string
}

export type Target = {
  id: string
  name: string
  targetType: TargetType
  state?: string
  [key: string]: unknown
}

export type PortPreviewUrl = {
  sandboxId: string
  url: string
  token: string
}

export type SignedPortPreviewUrl = PortPreviewUrl & {
  port: number
}

export type SshAccessDto = {
  id: string
  sandboxId: string
  token: string
  expiresAt: string
  createdAt: string
  updatedAt: string
  sshCommand: string
}

export type LogEntry = {
  timestamp: string
  body: string
  severityText: string
  severityNumber?: number
  serviceName: string
  resourceAttributes: Record<string, string>
  logAttributes: Record<string, string>
  traceId?: string
  spanId?: string
}

export type PaginatedLogs = {
  items: LogEntry[]
  total: number
  page: number
  totalPages: number
}

export type TraceSummary = {
  traceId: string
  rootSpanName: string
  startTime: string
  endTime: string
  durationMs: number
  totalDurationMs?: number
  spanCount: number
  statusCode?: string
}

export type PaginatedTraces = {
  items: TraceSummary[]
  total: number
  page: number
  totalPages: number
}

export type TraceSpan = {
  traceId: string
  spanId: string
  parentSpanId?: string
  spanName: string
  timestamp: string
  durationNs: number
  durationMs?: number
  spanAttributes: Record<string, string>
  statusCode?: string
  statusMessage?: string
}

export type MetricSeries = {
  metricName: string
  dataPoints: Array<{ timestamp: string; value: number }>
}

export type MetricsResponse = {
  series: MetricSeries[]
}

function cleanParams(params: Record<string, unknown>) {
  const search = new URLSearchParams()

  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') {
      continue
    }

    if (Array.isArray(value)) {
      for (const item of value) {
        if (item !== undefined && item !== null && item !== '') {
          search.append(key, item instanceof Date ? item.toISOString() : String(item))
        }
      }
      continue
    }

    search.set(key, value instanceof Date ? value.toISOString() : String(value))
  }

  const query = search.toString()
  return query ? `?${query}` : ''
}

class ResourceApi {
  protected readonly config: Configuration
  protected readonly axiosInstance: AxiosInstance

  constructor(config: Configuration, _basePath?: string, axiosInstance?: AxiosInstance) {
    this.config = config
    this.axiosInstance = axiosInstance ?? axios.create()
  }

  protected async request<T>(requestConfig: AxiosRequestConfig): ApiResponse<T> {
    const response = await this.axiosInstance.request<T>({
      baseURL: this.config.basePath,
      ...requestConfig,
    })
    return { data: response.data }
  }
}

export class SandboxApi extends ResourceApi {
  createSandbox(body: CreateSandbox): ApiResponse<Sandbox> {
    return this.request({ method: 'POST', url: '/sandbox', data: body })
  }

  listSandboxes(
    cursor?: string,
    limit?: number,
    _id?: string,
    name?: string,
    labels?: string,
    includeErroredDeleted?: boolean,
    states?: SandboxState[],
    snapshots?: string[],
    _targets?: string[],
    sandboxClasses?: SandboxClass[],
    minCpu?: number,
    maxCpu?: number,
    minMemoryGiB?: number,
    maxMemoryGiB?: number,
    minDiskGiB?: number,
    maxDiskGiB?: number,
    isPublic?: boolean,
    isRecoverable?: boolean,
    createdAtAfter?: Date,
    createdAtBefore?: Date,
    lastEventAfter?: Date,
    lastEventBefore?: Date,
    sort?: SandboxListSortField,
    order?: SandboxListSortDirection,
  ): ApiResponse<ListSandboxesResponse> {
    return this.request({
      method: 'GET',
      url:
        '/sandbox' +
        cleanParams({
          cursor,
          limit,
          name,
          labels,
          includeErroredDeleted,
          states,
          snapshots,
          sandboxClasses,
          minCpu,
          maxCpu,
          minMemoryGiB,
          maxMemoryGiB,
          minDiskGiB,
          maxDiskGiB,
          isPublic,
          isRecoverable,
          createdAtAfter,
          createdAtBefore,
          lastEventAfter,
          lastEventBefore,
          sort,
          order,
        }),
    })
  }

  getSandbox(sandboxIdOrName: string): ApiResponse<Sandbox> {
    return this.request({ method: 'GET', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}` })
  }

  deleteSandbox(sandboxIdOrName: string): ApiResponse<Sandbox> {
    return this.request({ method: 'DELETE', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}` })
  }

  startSandbox(sandboxIdOrName: string): ApiResponse<Sandbox> {
    return this.request({ method: 'POST', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/start` })
  }

  stopSandbox(sandboxIdOrName: string, force?: boolean): ApiResponse<Sandbox> {
    return this.request({
      method: 'POST',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/stop${cleanParams({ force })}`,
    })
  }

  pauseSandbox(sandboxIdOrName: string): ApiResponse<Sandbox> {
    return this.request({ method: 'POST', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/pause` })
  }

  recoverSandbox(sandboxIdOrName: string, skipStart?: boolean): ApiResponse<Sandbox> {
    return this.request({
      method: 'POST',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/recover${cleanParams({ skipStart })}`,
    })
  }

  archiveSandbox(sandboxIdOrName: string): ApiResponse<Sandbox> {
    return this.request({ method: 'POST', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/archive` })
  }

  createSandboxSnapshot(
    sandboxIdOrName: string,
    body: { name: string; includeMemory?: boolean },
  ): ApiResponse<Sandbox> {
    return this.request({
      method: 'POST',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/snapshot`,
      data: body,
    })
  }

  forkSandbox(sandboxIdOrName: string, body: object = {}): ApiResponse<Sandbox> {
    return this.request({ method: 'POST', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/fork`, data: body })
  }

  getSandboxForks(sandboxIdOrName: string, includeDestroyed?: boolean) {
    return this.request<Sandbox[]>({
      method: 'GET',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/forks${cleanParams({ includeDestroyed })}`,
    })
  }

  getSandboxAncestors(sandboxIdOrName: string): ApiResponse<Sandbox[]> {
    return this.request({ method: 'GET', url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/ancestors` })
  }

  getSignedPortPreviewUrl(
    sandboxIdOrName: string,
    port: number,
    expiresInSeconds?: number,
  ): ApiResponse<SignedPortPreviewUrl> {
    return this.request({
      method: 'GET',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/ports/${port}/signed-preview-url${cleanParams({
        expiresInSeconds,
      })}`,
    })
  }

  createSshAccess(sandboxIdOrName: string, expiresInMinutes?: number): ApiResponse<SshAccessDto> {
    return this.request({
      method: 'POST',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/ssh-access${cleanParams({ expiresInMinutes })}`,
    })
  }

  revokeSshAccess(sandboxIdOrName: string, token?: string): ApiResponse<Sandbox> {
    return this.request({
      method: 'DELETE',
      url: `/sandbox/${encodeURIComponent(sandboxIdOrName)}/ssh-access${cleanParams({
        token,
      })}`,
    })
  }
}

export class SnapshotsApi extends ResourceApi {
  createSnapshot(body: CreateSnapshot): ApiResponse<SnapshotDto> {
    return this.request({ method: 'POST', url: '/snapshots', data: body })
  }

  getSnapshot(snapshotIdOrName: string): ApiResponse<SnapshotDto> {
    return this.request({ method: 'GET', url: `/snapshots/${encodeURIComponent(snapshotIdOrName)}` })
  }

  getAllSnapshots(
    page?: number,
    limit?: number,
    name?: string,
    sort?: GetAllSnapshotsSortEnum,
    order?: GetAllSnapshotsOrderEnum,
  ): ApiResponse<PaginatedSnapshots> {
    return this.request({
      method: 'GET',
      url: `/snapshots${cleanParams({ page, limit, name, sort, order })}`,
    })
  }

  removeSnapshot(snapshotId: string): ApiResponse<void> {
    return this.request({ method: 'DELETE', url: `/snapshots/${encodeURIComponent(snapshotId)}` })
  }

  activateSnapshot(snapshotId: string): ApiResponse<SnapshotDto> {
    return this.request({ method: 'POST', url: `/snapshots/${encodeURIComponent(snapshotId)}/activate` })
  }

  deactivateSnapshot(snapshotId: string): ApiResponse<void> {
    return this.request({ method: 'POST', url: `/snapshots/${encodeURIComponent(snapshotId)}/deactivate` })
  }
}

export class VolumesApi extends ResourceApi {
  listVolumes(includeDeleted = false): ApiResponse<VolumeDto[]> {
    return this.request({ method: 'GET', url: `/volumes${cleanParams({ includeDeleted })}` })
  }

  createVolume(body: CreateVolume): ApiResponse<VolumeDto> {
    return this.request({ method: 'POST', url: '/volumes', data: body })
  }

  deleteVolume(volumeId: string): ApiResponse<void> {
    return this.request({ method: 'DELETE', url: `/volumes/${encodeURIComponent(volumeId)}` })
  }
}

export class RunnersApi extends ResourceApi {
  listRunners(target?: string): ApiResponse<Runner[]> {
    return this.request<Runner[]>({ method: 'GET', url: `/runners${cleanParams({ target })}` }).then((response) => ({
      data: response.data.map((runner) => ({
        ...runner,
        target: runner.target ?? runner.target,
        appVersion: runner.appVersion ?? runner.version,
      })),
    }))
  }

  createRunner(body: CreateRunner): ApiResponse<CreateRunnerResponse> {
    const { target, ...rest } = body
    return this.request({ method: 'POST', url: '/runners', data: { ...rest, target: body.target ?? target } })
  }

  updateRunnerScheduling(
    runnerId: string,
    body?: { data?: { unschedulable: boolean }; unschedulable?: boolean },
  ): ApiResponse<Runner> {
    return this.request({
      method: 'PATCH',
      url: `/runners/${encodeURIComponent(runnerId)}/scheduling`,
      data: { unschedulable: body?.data?.unschedulable ?? body?.unschedulable },
    })
  }

  deleteRunner(runnerId: string): ApiResponse<void> {
    return this.request({ method: 'DELETE', url: `/runners/${encodeURIComponent(runnerId)}` })
  }
}

export class DockerRegistryApi extends ResourceApi {
  listRegistries(): ApiResponse<DockerRegistry[]> {
    return this.request({ method: 'GET', url: '/docker-registry' })
  }

  createRegistry(body: CreateDockerRegistry): ApiResponse<DockerRegistry> {
    return this.request({ method: 'POST', url: '/docker-registry', data: body })
  }

  updateRegistry(registryId: string, body: UpdateDockerRegistry): ApiResponse<DockerRegistry> {
    return this.request({ method: 'PATCH', url: `/docker-registry/${encodeURIComponent(registryId)}`, data: body })
  }

  deleteRegistry(registryId: string): ApiResponse<void> {
    return this.request({ method: 'DELETE', url: `/docker-registry/${encodeURIComponent(registryId)}` })
  }
}

export class SandboxTelemetryApi extends ResourceApi {
  async sandboxSandboxIdTelemetryLogsGet(
    sandboxId: string,
    from: string,
    to: string,
    severities?: string,
    search?: string,
    limit?: number,
    offset?: number,
  ): ApiResponse<LogEntry[]> {
    const page = limit && offset ? Math.floor(offset / limit) + 1 : undefined
    const response = await this.request<PaginatedLogs>({
      method: 'GET',
      url: `/sandbox/${encodeURIComponent(sandboxId)}/telemetry/logs${cleanParams({
        from,
        to,
        severities,
        search,
        limit,
        page,
      })}`,
    })
    return { data: response.data.items ?? [] }
  }

  async sandboxSandboxIdTelemetryTracesGet(
    sandboxId: string,
    from: string,
    to: string,
    limit?: number,
    offset?: number,
  ): ApiResponse<TraceSummary[]> {
    const page = limit && offset ? Math.floor(offset / limit) + 1 : undefined
    const response = await this.request<PaginatedTraces>({
      method: 'GET',
      url: `/sandbox/${encodeURIComponent(sandboxId)}/telemetry/traces${cleanParams({ from, to, limit, page })}`,
    })
    return { data: response.data.items ?? [] }
  }

  sandboxSandboxIdTelemetryTracesTraceIdGet(sandboxId: string, traceId: string): ApiResponse<TraceSpan[]> {
    return this.request({
      method: 'GET',
      url: `/sandbox/${encodeURIComponent(sandboxId)}/telemetry/traces/${encodeURIComponent(traceId)}`,
    })
  }

  async sandboxSandboxIdTelemetryMetricsGet(
    sandboxId: string,
    from: string,
    to: string,
    metricNames?: string,
  ): ApiResponse<Array<{ metricName: string; timestamp: string; value: number }>> {
    const response = await this.request<MetricsResponse>({
      method: 'GET',
      url: `/sandbox/${encodeURIComponent(sandboxId)}/telemetry/metrics${cleanParams({
        from,
        to,
        metricNames,
      })}`,
    })

    return {
      data: (response.data.series ?? []).flatMap((series) =>
        (series.dataPoints ?? []).map((point) => ({ metricName: series.metricName, ...point })),
      ),
    }
  }
}

export class ToolboxApi {
  private readonly statuses = new Map<string, string>()

  async getComputerUseStatusDeprecated(sandboxId: string): ApiResponse<{ status: string }> {
    return { data: { status: this.statuses.get(sandboxId) ?? 'stopped' } }
  }

  async startComputerUseDeprecated(sandboxId: string): ApiResponse<{ status: string }> {
    this.statuses.set(sandboxId, 'active')
    return { data: { status: 'active' } }
  }
}
