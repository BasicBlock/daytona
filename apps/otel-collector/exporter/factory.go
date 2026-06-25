// Copyright Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package exporter

import (
	"context"
	"net/http"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"

	common_cache "github.com/daytonaio/common-go/pkg/cache"
	resolverconfig "github.com/daytonaio/otel-collector/exporter/internal/config"
)

const (
	typeStr   = "daytona_exporter"
	stability = component.StabilityLevelBeta
)

// NewFactory creates a factory for the custom exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		component.MustNewType(typeStr),
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, stability),
		exporter.WithMetrics(createMetricsExporter, stability),
		exporter.WithLogs(createLogsExporter, stability),
	)
}

// createDefaultConfig creates the default configuration for the exporter.
func createDefaultConfig() component.Config {
	return &Config{
		SandboxAuthTokenHeader: "sandbox-auth-token",
		CacheTTL:               5 * time.Minute,
		DefaultTimeout:         30 * time.Second,
		RetrySettings:          configretry.NewDefaultBackOffConfig(),
		SendingQueue:           exporterhelper.NewDefaultQueueConfig(),
	}
}

// createTracesExporter creates a new trace exporter.
func createTracesExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Traces, error) {
	c := cfg.(*Config)

	var cache common_cache.ICache[resolverconfig.OtelConfig]

	if c.Redis != nil {
		redisCache, err := common_cache.NewRedisCache[resolverconfig.OtelConfig](c.Redis, "sandbox-otel-config:")
		if err != nil {
			return nil, err
		}
		cache = redisCache
	} else {
		cache = common_cache.NewMapCache[resolverconfig.OtelConfig](ctx)
	}

	resolver := resolverconfig.NewResolver(cache, set.Logger, c.ApiUrl, &http.Client{Transport: http.DefaultTransport}, c.CacheTTL)

	te := newTracesExporter(exporterConfig{
		config:   c,
		resolver: resolver,
		logger:   set.Logger,
	})

	return exporterhelper.NewTraces(
		ctx,
		set,
		cfg,
		te.push,
		exporterhelper.WithRetry(c.RetrySettings),
		exporterhelper.WithQueue(configoptional.Some(c.SendingQueue)),
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: c.DefaultTimeout}),
		exporterhelper.WithShutdown(te.shutdown),
	)
}

// createMetricsExporter creates a new metrics exporter.
func createMetricsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Metrics, error) {
	c := cfg.(*Config)

	var cache common_cache.ICache[resolverconfig.OtelConfig]
	// Create cache and resolver
	if c.Redis != nil {
		redisCache, err := common_cache.NewRedisCache[resolverconfig.OtelConfig](c.Redis, "sandbox-otel-config:")
		if err != nil {
			return nil, err
		}
		cache = redisCache
	} else {
		cache = common_cache.NewMapCache[resolverconfig.OtelConfig](ctx)
	}
	resolver := resolverconfig.NewResolver(cache, set.Logger, c.ApiUrl, &http.Client{Transport: http.DefaultTransport}, c.CacheTTL)

	me := newMetricExporter(exporterConfig{
		config:   c,
		resolver: resolver,
		logger:   set.Logger,
	})

	return exporterhelper.NewMetrics(
		ctx,
		set,
		cfg,
		me.push,
		exporterhelper.WithRetry(c.RetrySettings),
		exporterhelper.WithQueue(configoptional.Some(c.SendingQueue)),
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: c.DefaultTimeout}),
		exporterhelper.WithShutdown(me.shutdown),
	)
}

// createLogsExporter creates a new logs exporter.
func createLogsExporter(
	ctx context.Context,
	set exporter.Settings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)

	// Create cache and resolver
	var cache common_cache.ICache[resolverconfig.OtelConfig]
	if c.Redis != nil {
		redisCache, err := common_cache.NewRedisCache[resolverconfig.OtelConfig](c.Redis, "sandbox-otel-config:")
		if err != nil {
			return nil, err
		}
		cache = redisCache
	} else {
		cache = common_cache.NewMapCache[resolverconfig.OtelConfig](ctx)
	}
	resolver := resolverconfig.NewResolver(cache, set.Logger, c.ApiUrl, &http.Client{Transport: http.DefaultTransport}, c.CacheTTL)

	le := newLogsExporter(exporterConfig{
		config:   c,
		resolver: resolver,
		logger:   set.Logger,
	})

	return exporterhelper.NewLogs(
		ctx,
		set,
		cfg,
		le.push,
		exporterhelper.WithRetry(c.RetrySettings),
		exporterhelper.WithQueue(configoptional.Some(c.SendingQueue)),
		exporterhelper.WithTimeout(exporterhelper.TimeoutConfig{Timeout: c.DefaultTimeout}),
		exporterhelper.WithShutdown(le.shutdown),
	)
}
