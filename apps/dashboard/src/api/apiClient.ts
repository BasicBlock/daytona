/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { DashboardConfig } from '@/types/DashboardConfig'
import {
  Configuration,
  DockerRegistryApi,
  RunnersApi,
  SandboxApi,
  SandboxTelemetryApi,
  SnapshotsApi,
  ToolboxApi,
  VolumesApi,
} from '@daytona/api-client'
import axios, { AxiosError } from 'axios'
import { DaytonaError } from './errors'

export class ApiClient {
  private config: Configuration
  private axios: ReturnType<typeof axios.create>
  private _snapshotApi: SnapshotsApi
  private _sandboxApi: SandboxApi
  private _dockerRegistryApi: DockerRegistryApi
  private _volumeApi: VolumesApi
  private _toolboxApi: ToolboxApi
  private _runnersApi: RunnersApi
  private _analyticsTelemetryApi: SandboxTelemetryApi

  constructor(config: DashboardConfig) {
    this.config = new Configuration({
      basePath: config.apiUrl,
    })

    this.axios = axios.create()
    this.axios.interceptors.response.use(
      (response) => response,
      (error) => {
        let errorMessage: string

        if (error instanceof AxiosError && error.message.includes('timeout of')) {
          errorMessage = 'Operation timed out'
        } else {
          errorMessage = error.response?.data?.message || error.response?.data || error.message || String(error)
        }

        throw DaytonaError.fromString(String(errorMessage), { cause: error instanceof Error ? error : undefined })
      },
    )

    this._snapshotApi = new SnapshotsApi(this.config, undefined, this.axios)
    this._sandboxApi = new SandboxApi(this.config, undefined, this.axios)
    this._dockerRegistryApi = new DockerRegistryApi(this.config, undefined, this.axios)
    this._volumeApi = new VolumesApi(this.config, undefined, this.axios)
    this._toolboxApi = new ToolboxApi()
    this._runnersApi = new RunnersApi(this.config, undefined, this.axios)
    this._analyticsTelemetryApi = new SandboxTelemetryApi(this.config, undefined, this.axios)
  }

  public get snapshotApi() {
    return this._snapshotApi
  }

  public get sandboxApi() {
    return this._sandboxApi
  }

  public get dockerRegistryApi() {
    return this._dockerRegistryApi
  }

  public get volumeApi() {
    return this._volumeApi
  }

  public get toolboxApi() {
    return this._toolboxApi
  }

  public get runnersApi() {
    return this._runnersApi
  }

  public get analyticsTelemetryApi() {
    return this._analyticsTelemetryApi
  }

  public get analyticsUsageApi() {
    return null
  }

  public get axiosInstance() {
    return axios.create({
      baseURL: this.config.basePath,
    })
  }
}
