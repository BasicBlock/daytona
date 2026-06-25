/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { SandboxClass } from '../sandbox/enums/sandbox-class.enum'

const configuration = {
  production: process.env.NODE_ENV === 'production',
  version: process.env.VERSION || '0.0.0-dev',
  environment: process.env.ENVIRONMENT,
  runMigrations: process.env.RUN_MIGRATIONS === 'true',
  port: parseInt(process.env.PORT, 10),
  appUrl: process.env.APP_URL,
  database: {
    host: process.env.DB_HOST,
    port: parseInt(process.env.DB_PORT || '5432', 10),
    username: process.env.DB_USERNAME,
    password: process.env.DB_PASSWORD,
    database: process.env.DB_DATABASE,
    tls: {
      enabled: process.env.DB_TLS_ENABLED === 'true',
      rejectUnauthorized: process.env.DB_TLS_REJECT_UNAUTHORIZED !== 'false',
    },
    pool: {
      max: process.env.DB_POOL_MAX && parseInt(process.env.DB_POOL_MAX, 10),
      min: process.env.DB_POOL_MIN && parseInt(process.env.DB_POOL_MIN, 10),
      idleTimeoutMillis: process.env.DB_POOL_IDLE_TIMEOUT_MS && parseInt(process.env.DB_POOL_IDLE_TIMEOUT_MS, 10),
      connectionTimeoutMillis:
        process.env.DB_POOL_CONNECTION_TIMEOUT_MS && parseInt(process.env.DB_POOL_CONNECTION_TIMEOUT_MS, 10),
    },
  },
  redis: {
    host: process.env.REDIS_HOST,
    port: parseInt(process.env.REDIS_PORT || '6379', 10),
    username: process.env.REDIS_USERNAME,
    password: process.env.REDIS_PASSWORD,
    tls: process.env.REDIS_TLS === 'true' ? {} : undefined,
  },
  defaultSnapshot: process.env.DEFAULT_SNAPSHOT,
  defaultSandboxClass: SandboxClass.LINUX_VM,
  dashboardUrl: process.env.DASHBOARD_URL,
  // Default to empty string - dashboard will then hit '/api'
  dashboardBaseApiUrl: process.env.DASHBOARD_BASE_API_URL || '',
  transientRegistry: {
    url: process.env.TRANSIENT_REGISTRY_URL,
    admin: process.env.TRANSIENT_REGISTRY_ADMIN,
    password: process.env.TRANSIENT_REGISTRY_PASSWORD,
    projectId: process.env.TRANSIENT_REGISTRY_PROJECT_ID,
  },
  internalRegistry: {
    url: process.env.INTERNAL_REGISTRY_URL,
    admin: process.env.INTERNAL_REGISTRY_ADMIN,
    password: process.env.INTERNAL_REGISTRY_PASSWORD,
    projectId: process.env.INTERNAL_REGISTRY_PROJECT_ID,
  },
  ecr: {
    brokerRoleArn: process.env.ECR_BROKER_ROLE_ARN,
  },
  s3: {
    endpoint: process.env.S3_ENDPOINT,
    stsEndpoint: process.env.S3_STS_ENDPOINT,
    region: process.env.S3_REGION,
    accessKey: process.env.S3_ACCESS_KEY,
    secretKey: process.env.S3_SECRET_KEY,
    defaultBucket: process.env.S3_DEFAULT_BUCKET,
    accountId: process.env.S3_ACCOUNT_ID,
    roleName: process.env.S3_ROLE_NAME,
  },
  skipConnections: process.env.SKIP_CONNECTIONS === 'true',
  maxAutoArchiveInterval: parseInt(process.env.MAX_AUTO_ARCHIVE_INTERVAL || '43200', 10),
  maintananceMode: process.env.MAINTENANCE_MODE === 'true',
  disableCronJobs: process.env.DISABLE_CRON_JOBS === 'true',
  appRole: process.env.APP_ROLE || 'all',
  proxy: {
    domain: process.env.PROXY_DOMAIN,
    protocol: process.env.PROXY_PROTOCOL,
    templateUrl: process.env.PROXY_TEMPLATE_URL,
    toolboxUrl:
      (process.env.PROXY_TOOLBOX_BASE_URL || `${process.env.PROXY_PROTOCOL}://${process.env.PROXY_DOMAIN}`) +
      '/toolbox',
  },
  kafka: {
    enabled: process.env.KAFKA_ENABLED === 'true',
    brokers: process.env.KAFKA_BROKERS || 'localhost:9092',
    clientId: process.env.KAFKA_CLIENT_ID,
    sasl: {
      mechanism: process.env.KAFKA_SASL_MECHANISM,
      username: process.env.KAFKA_SASL_USERNAME,
      password: process.env.KAFKA_SASL_PASSWORD,
    },
    tls: {
      enabled: process.env.KAFKA_TLS_ENABLED === 'true',
      rejectUnauthorized: process.env.KAFKA_TLS_REJECT_UNAUTHORIZED !== 'false',
    },
  },
  opensearch: {
    nodes: process.env.OPENSEARCH_NODES || 'https://localhost:9200',
    username: process.env.OPENSEARCH_USERNAME,
    password: process.env.OPENSEARCH_PASSWORD,
    aws: {
      roleArn: process.env.OPENSEARCH_AWS_ROLE_ARN,
      region: process.env.OPENSEARCH_AWS_REGION,
    },
    tls: {
      rejectUnauthorized: process.env.OPENSEARCH_TLS_REJECT_UNAUTHORIZED !== 'false',
    },
  },
  sandboxSearch: {
    publish: {
      enabled: process.env.SANDBOX_SEARCH_PUBLISH_ENABLED === 'true',
      storageAdapter: process.env.SANDBOX_SEARCH_PUBLISH_STORAGE_ADAPTER || 'opensearch',
      opensearchIndexName: process.env.SANDBOX_SEARCH_PUBLISH_OPENSEARCH_INDEX_NAME || 'sandboxes',
      numberOfShards: parseInt(process.env.SANDBOX_SEARCH_PUBLISH_NUMBER_OF_SHARDS || '1', 10),
      numberOfReplicas: parseInt(process.env.SANDBOX_SEARCH_PUBLISH_NUMBER_OF_REPLICAS || '1', 10),
    },
  },
  cronTimeZone: process.env.CRON_TIMEZONE,
  maxConcurrentBackupsPerRunner: parseInt(process.env.MAX_CONCURRENT_BACKUPS_PER_RUNNER || '6', 10),
  backupRetryIntervalHours: parseInt(process.env.BACKUP_RETRY_INTERVAL_HOURS || '6', 10),
  sshGateway: {
    command: process.env.SSH_GATEWAY_COMMAND,
    publicKey: process.env.SSH_GATEWAY_PUBLIC_KEY,
    url: process.env.SSH_GATEWAY_URL,
  },
  defaultRunner: {
    domain: process.env.DEFAULT_RUNNER_DOMAIN,
    proxyUrl: process.env.DEFAULT_RUNNER_PROXY_URL,
    apiUrl: process.env.DEFAULT_RUNNER_API_URL,
    cpu: parseInt(process.env.DEFAULT_RUNNER_CPU || '4', 10),
    memory: parseInt(process.env.DEFAULT_RUNNER_MEMORY || '8', 10),
    disk: parseInt(process.env.DEFAULT_RUNNER_DISK || '50', 10),
    apiVersion: '2' as const,
    name: process.env.DEFAULT_RUNNER_NAME,
  },
  buildInfo: {
    maxCpuPerRunner: parseInt(process.env.BUILD_INFO_MAX_CPU_PER_RUNNER || '40', 10),
  },
  runnerScore: {
    thresholds: {
      declarativeBuild: parseInt(process.env.RUNNER_DECLARATIVE_BUILD_SCORE_THRESHOLD || '10', 10),
      availability: parseInt(process.env.RUNNER_AVAILABILITY_SCORE_THRESHOLD || '10', 10),
      start: parseInt(process.env.RUNNER_START_SCORE_THRESHOLD || '3', 10),
      initialRunnerScoreAddon: parseInt(process.env.RUNNER_INITIAL_RUNNER_SCORE_ADDON || '20', 10),
    },
    weights: {
      cpuUsage: parseFloat(process.env.RUNNER_CPU_USAGE_WEIGHT || '0.25'),
      memoryUsage: parseFloat(process.env.RUNNER_MEMORY_USAGE_WEIGHT || '0.4'),
      diskUsage: parseFloat(process.env.RUNNER_DISK_USAGE_WEIGHT || '0.4'),
      allocatedCpu: parseFloat(process.env.RUNNER_ALLOCATED_CPU_WEIGHT || '0.03'),
      allocatedMemory: parseFloat(process.env.RUNNER_ALLOCATED_MEMORY_WEIGHT || '0.03'),
      allocatedDisk: parseFloat(process.env.RUNNER_ALLOCATED_DISK_WEIGHT || '0.03'),
      startedSandboxes: parseFloat(process.env.RUNNER_STARTED_SANDBOXES_WEIGHT || '0.1'),
    },
    penalty: {
      exponents: {
        cpuLoadAvg: parseFloat(process.env.RUNNER_CPU_LOAD_AVG_PENALTY_EXPONENT || '0.1'),
        cpu: parseFloat(process.env.RUNNER_CPU_PENALTY_EXPONENT || '0.15'),
        memory: parseFloat(process.env.RUNNER_MEMORY_PENALTY_EXPONENT || '0.15'),
        disk: parseFloat(process.env.RUNNER_DISK_PENALTY_EXPONENT || '0.15'),
      },
      thresholds: {
        // cpuLoadAvg is a normalized per-CPU load average (e.g. load_avg / num_cpus), not a percentage like the cpu/memory/disk thresholds below.
        cpuLoadAvg: parseFloat(process.env.RUNNER_CPU_LOAD_AVG_PENALTY_THRESHOLD || '0.7'),
        cpu: parseInt(process.env.RUNNER_CPU_PENALTY_THRESHOLD || '90', 10),
        memory: parseInt(process.env.RUNNER_MEMORY_PENALTY_THRESHOLD || '75', 10),
        disk: parseInt(process.env.RUNNER_DISK_PENALTY_THRESHOLD || '75', 10),
      },
    },
    targetValues: {
      optimal: {
        cpu: parseInt(process.env.RUNNER_OPTIMAL_CPU || '0', 10),
        memory: parseInt(process.env.RUNNER_OPTIMAL_MEMORY || '0', 10),
        disk: parseInt(process.env.RUNNER_OPTIMAL_DISK || '0', 10),
        allocCpu: parseInt(process.env.RUNNER_OPTIMAL_ALLOC_CPU || '100', 10),
        allocMem: parseInt(process.env.RUNNER_OPTIMAL_ALLOC_MEM || '100', 10),
        allocDisk: parseInt(process.env.RUNNER_OPTIMAL_ALLOC_DISK || '100', 10),
        startedSandboxes: parseInt(process.env.RUNNER_OPTIMAL_STARTED_SANDBOXES || '0', 10),
      },
      critical: {
        cpu: parseInt(process.env.RUNNER_CRITICAL_CPU || '100', 10),
        memory: parseInt(process.env.RUNNER_CRITICAL_MEMORY || '100', 10),
        disk: parseInt(process.env.RUNNER_CRITICAL_DISK || '100', 10),
        allocCpu: parseInt(process.env.RUNNER_CRITICAL_ALLOC_CPU || '500', 10),
        allocMem: parseInt(process.env.RUNNER_CRITICAL_ALLOC_MEM || '500', 10),
        allocDisk: parseInt(process.env.RUNNER_CRITICAL_ALLOC_DISK || '500', 10),
        startedSandboxes: parseInt(process.env.RUNNER_CRITICAL_STARTED_SANDBOXES || '100', 10),
      },
    },
  },
  log: {
    console: {
      disabled: process.env.LOG_CONSOLE_DISABLED === 'true',
    },
    level: process.env.LOG_LEVEL || 'info',
    requests: {
      enabled: process.env.LOG_REQUESTS_ENABLED === 'true',
    },
  },
  defaultTarget: {
    id: process.env.DEFAULT_TARGET_ID || 'us',
  },
  runnerHealthTimeout: parseInt(process.env.RUNNER_HEALTH_TIMEOUT_SECONDS || '3', 10),
  warmPool: {
    candidateLimit: parseInt(process.env.WARM_POOL_CANDIDATE_LIMIT || '300', 10),
  },
  webhook: {
    authToken: process.env.SVIX_AUTH_TOKEN,
    serverUrl: process.env.SVIX_SERVER_URL,
  },
  otelCollector: {
    endpointUrl: process.env.OTEL_COLLECTOR_ENDPOINT_URL || process.env.SANDBOX_OTEL_ENDPOINT_URL,
    forwardEndpointUrl: process.env.SANDBOX_OTEL_FORWARD_ENDPOINT_URL || process.env.OTEL_FORWARD_ENDPOINT_URL,
    forwardHeaders: process.env.SANDBOX_OTEL_FORWARD_HEADERS || process.env.OTEL_FORWARD_HEADERS,
  },
  clickhouse: {
    host: process.env.CLICKHOUSE_HOST,
    port: parseInt(process.env.CLICKHOUSE_PORT || '8123', 10),
    database: process.env.CLICKHOUSE_DATABASE || 'otel',
    username: process.env.CLICKHOUSE_USERNAME || 'default',
    password: process.env.CLICKHOUSE_PASSWORD,
    protocol: process.env.CLICKHOUSE_PROTOCOL || 'https',
  },
  sandboxActivity: {
    throttleTtlSeconds: parseInt(process.env.SANDBOX_ACTIVITY_THROTTLE_TTL_SECONDS || '5', 10),
    flushBatchSize: parseInt(process.env.SANDBOX_ACTIVITY_FLUSH_BATCH_SIZE || '1000', 10),
  },
  encryption: {
    key: process.env.ENCRYPTION_KEY,
    salt: process.env.ENCRYPTION_SALT,
  },
  sandboxSnapshottingTimeoutMin: parseInt(process.env.SANDBOX_SNAPSHOTTING_TIMEOUT_MIN || '60', 10),
  failedSnapshotRunnerRetentionHours: parseInt(process.env.FAILED_SNAPSHOT_RUNNER_RETENTION_HOURS || '3', 10),
  buildInfoSnapshotRunnerStalenessDays: parseInt(process.env.BUILDINFO_SNAPSHOT_RUNNER_STALENESS_DAYS || '7', 10),
  // DRAINING_MODE: 'migrate' (default) relocates stopped sandboxes to another runner;
  // 'archive' archives in place (no target capacity needed — for k8s full drains).
  // DRAINING_FORCE: when true, force-stops running sandboxes so the drain converges.
  draining: {
    mode: (process.env.DRAINING_MODE === 'archive' ? 'archive' : 'migrate') as 'migrate' | 'archive',
    force: process.env.DRAINING_FORCE === 'true',
  },
  dontServeDashboard: process.env.DONT_SERVE_DASHBOARD === 'true',
}

export { configuration }
