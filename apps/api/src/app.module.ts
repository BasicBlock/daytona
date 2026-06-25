/*
 * Copyright 2025 Daytona Platforms Inc.
 * SPDX-License-Identifier: AGPL-3.0
 */

import { Module, NestModule, MiddlewareConsumer, RequestMethod } from '@nestjs/common'
import { VersionHeaderMiddleware } from './common/middleware/version-header.middleware'
import { TypeOrmModule } from '@nestjs/typeorm'
import { SandboxModule } from './sandbox/sandbox.module'
import { ServeStaticModule } from '@nestjs/serve-static'
import { join } from 'path'
import { DockerRegistryModule } from './docker-registry/docker-registry.module'
import { RedisModule } from '@nestjs-modules/ioredis'
import { ScheduleModule } from '@nestjs/schedule'
import { EventEmitterModule } from '@nestjs/event-emitter'
import { TypedConfigService } from './config/typed-config.service'
import { TypedConfigModule } from './config/typed-config.module'
import { ObjectStorageModule } from './object-storage/object-storage.module'
import { CustomNamingStrategy } from './common/utils/naming-strategy.util'
import { MaintenanceMiddleware } from './common/middleware/maintenance.middleware'
import { HealthModule } from './health/health.module'
import { LoggerModule } from 'nestjs-pino'
import { getPinoTransport, swapMessageAndObject } from './common/utils/pino.util'
import { BodyParserErrorModule } from './common/modules/body-parser-error.module'
import { ClickHouseModule } from './clickhouse/clickhouse.module'
import { SandboxTelemetryModule } from './sandbox-telemetry/sandbox-telemetry.module'
import { WebhookModule } from './webhook/webhook.module'

@Module({
  imports: [
    LoggerModule.forRootAsync({
      useFactory: (configService: TypedConfigService) => {
        const logConfig = configService.get('log')
        const isProduction = configService.get('production')
        return {
          pinoHttp: {
            autoLogging: logConfig.requests.enabled,
            level: logConfig.level,
            hooks: {
              logMethod: swapMessageAndObject,
            },
            quietReqLogger: true,
            transport: getPinoTransport(isProduction, logConfig),
          },
        }
      },
      inject: [TypedConfigService],
    }),
    TypedConfigModule.forRoot({
      isGlobal: true,
    }),
    TypeOrmModule.forRootAsync({
      inject: [TypedConfigService],
      useFactory: (configService: TypedConfigService) => {
        return {
          type: 'postgres',
          host: configService.getOrThrow('database.host'),
          port: configService.getOrThrow('database.port'),
          username: configService.getOrThrow('database.username'),
          password: configService.getOrThrow('database.password'),
          database: configService.getOrThrow('database.database'),
          autoLoadEntities: true,
          migrations: [join(__dirname, 'migrations/**/*-migration.{ts,js}')],
          migrationsRun: configService.get('runMigrations') || !configService.getOrThrow('production'),
          namingStrategy: new CustomNamingStrategy(),
          manualInitialization: configService.get('skipConnections'),
          ssl: configService.get('database.tls.enabled')
            ? {
                rejectUnauthorized: configService.get('database.tls.rejectUnauthorized'),
              }
            : undefined,
          extra: {
            max: configService.get('database.pool.max'),
            min: configService.get('database.pool.min'),
            idleTimeoutMillis: configService.get('database.pool.idleTimeoutMillis'),
            connectionTimeoutMillis: configService.get('database.pool.connectionTimeoutMillis'),
          },
          cache: {
            type: 'ioredis',
            ignoreErrors: true,
            options: configService.getRedisConfig({ keyPrefix: 'typeorm:' }),
          },
          entitySkipConstructor: true,
        }
      },
    }),
    BodyParserErrorModule,
    ServeStaticModule.forRoot({
      rootPath: join(__dirname, '..'),
      exclude: ['/api/{*path}'],
      renderPath: '/runner-amd64',
      serveStaticOptions: {
        cacheControl: false,
      },
    }),
    ServeStaticModule.forRootAsync({
      inject: [TypedConfigService],
      useFactory: (configService: TypedConfigService) => {
        if (configService.get('dontServeDashboard')) {
          return []
        }
        return [
          {
            rootPath: join(__dirname, '..', 'dashboard'),
            serveRoot: '/dashboard',
            exclude: ['/api/{*path}'],
            renderPath: '/',
            serveStaticOptions: {
              cacheControl: false,
            },
          },
        ]
      },
    }),
    RedisModule.forRootAsync({
      inject: [TypedConfigService],
      useFactory: (configService: TypedConfigService) => ({
        type: 'single',
        options: configService.getRedisConfig(),
      }),
    }),
    EventEmitterModule.forRoot({
      maxListeners: 100,
    }),
    SandboxModule,
    DockerRegistryModule,
    ScheduleModule.forRoot(),
    ObjectStorageModule,
    HealthModule,
    ClickHouseModule,
    SandboxTelemetryModule,
    WebhookModule,
  ],
  controllers: [],
  providers: [],
})
export class AppModule implements NestModule {
  configure(consumer: MiddlewareConsumer) {
    consumer.apply(VersionHeaderMiddleware).forRoutes({ path: '{*path}', method: RequestMethod.ALL })
    consumer.apply(MaintenanceMiddleware).forRoutes({ path: '{*path}', method: RequestMethod.ALL })
  }
}
