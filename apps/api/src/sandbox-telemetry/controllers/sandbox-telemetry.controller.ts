/*
 * Copyright Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Controller, Get, Headers, NotFoundException, Param, Query } from '@nestjs/common'
import { ApiResponse, ApiOperation, ApiParam, ApiTags } from '@nestjs/swagger'
import { SandboxTelemetryService } from '../services/sandbox-telemetry.service'
import { LogsQueryParamsDto, TelemetryQueryParamsDto, MetricsQueryParamsDto } from '../dto/telemetry-query-params.dto'
import { PaginatedLogsDto } from '../dto/paginated-logs.dto'
import { PaginatedTracesDto } from '../dto/paginated-traces.dto'
import { TraceSpanDto } from '../dto/trace-span.dto'
import { MetricsResponseDto } from '../dto/metrics-response.dto'
import { OtelForwardingConfigDto } from '../dto/otel-forwarding-config.dto'

@Controller('sandbox')
@ApiTags('sandbox')
export class SandboxTelemetryController {
  constructor(private readonly sandboxTelemetryService: SandboxTelemetryService) {}

  @Get('telemetry/otel-config')
  @ApiOperation({
    summary: 'Get sandbox OTEL forwarding config',
    operationId: 'getSandboxOtelForwardingConfig',
    description: 'Retrieve the OTEL forwarding configuration for a sandbox auth token',
  })
  @ApiResponse({
    status: 200,
    description: 'OTEL forwarding configuration',
    type: OtelForwardingConfigDto,
  })
  async getSandboxOtelForwardingConfig(
    @Headers('sandbox-auth-token') sandboxAuthToken?: string,
  ): Promise<OtelForwardingConfigDto> {
    const config = await this.sandboxTelemetryService.getForwardingConfigBySandboxAuthToken(sandboxAuthToken)
    if (!config) {
      throw new NotFoundException()
    }
    return config
  }

  @Get(':sandboxId/telemetry/logs')
  @ApiOperation({
    summary: 'Get sandbox logs',
    operationId: 'getSandboxLogs',
    description: 'Retrieve OTEL logs for a sandbox within a time range',
  })
  @ApiParam({
    name: 'sandboxId',
    description: 'ID of the sandbox',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Paginated list of log entries',
    type: PaginatedLogsDto,
  })
  async getSandboxLogs(
    @Param('sandboxId') sandboxId: string,
    @Query() queryParams: LogsQueryParamsDto,
  ): Promise<PaginatedLogsDto> {
    return this.sandboxTelemetryService.getLogs(
      sandboxId,
      queryParams.from,
      queryParams.to,
      queryParams.page ?? 1,
      queryParams.limit ?? 100,
      queryParams.severities,
      queryParams.search,
    )
  }

  @Get(':sandboxId/telemetry/traces')
  @ApiOperation({
    summary: 'Get sandbox traces',
    operationId: 'getSandboxTraces',
    description: 'Retrieve OTEL traces for a sandbox within a time range',
  })
  @ApiParam({
    name: 'sandboxId',
    description: 'ID of the sandbox',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Paginated list of trace summaries',
    type: PaginatedTracesDto,
  })
  async getSandboxTraces(
    @Param('sandboxId') sandboxId: string,
    @Query() queryParams: TelemetryQueryParamsDto,
  ): Promise<PaginatedTracesDto> {
    return this.sandboxTelemetryService.getTraces(
      sandboxId,
      queryParams.from,
      queryParams.to,
      queryParams.page ?? 1,
      queryParams.limit ?? 100,
    )
  }

  @Get(':sandboxId/telemetry/traces/:traceId')
  @ApiOperation({
    summary: 'Get trace spans',
    operationId: 'getSandboxTraceSpans',
    description: 'Retrieve all spans for a specific trace',
  })
  @ApiParam({
    name: 'sandboxId',
    description: 'ID of the sandbox',
    type: 'string',
  })
  @ApiParam({
    name: 'traceId',
    description: 'ID of the trace',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'List of spans in the trace',
    type: [TraceSpanDto],
  })
  async getSandboxTraceSpans(
    @Param('sandboxId') sandboxId: string,
    @Param('traceId') traceId: string,
  ): Promise<TraceSpanDto[]> {
    return this.sandboxTelemetryService.getTraceSpans(sandboxId, traceId)
  }

  @Get(':sandboxId/telemetry/metrics')
  @ApiOperation({
    summary: 'Get sandbox metrics',
    operationId: 'getSandboxMetrics',
    description: 'Retrieve OTEL metrics for a sandbox within a time range',
  })
  @ApiParam({
    name: 'sandboxId',
    description: 'ID of the sandbox',
    type: 'string',
  })
  @ApiResponse({
    status: 200,
    description: 'Metrics time series data',
    type: MetricsResponseDto,
  })
  async getSandboxMetrics(
    @Param('sandboxId') sandboxId: string,
    @Query() queryParams: MetricsQueryParamsDto,
  ): Promise<MetricsResponseDto> {
    return this.sandboxTelemetryService.getMetrics(sandboxId, queryParams.from, queryParams.to, queryParams.metricNames)
  }
}
