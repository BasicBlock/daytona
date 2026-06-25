// Copyright Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	common_cache "github.com/daytonaio/common-go/pkg/cache"
	"go.uber.org/zap"
)

type OtelConfig struct {
	Endpoint string            `json:"endpoint"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// Resolver handles retrieving and caching endpoint configurations.
type Resolver struct {
	cache      common_cache.ICache[OtelConfig]
	logger     *zap.Logger
	apiURL     string
	httpClient *http.Client
	cacheTTL   time.Duration
}

// NewResolver creates a new configuration resolver.
func NewResolver(cache common_cache.ICache[OtelConfig], logger *zap.Logger, apiURL string, httpClient *http.Client, cacheTTL time.Duration) *Resolver {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &Resolver{
		cache:      cache,
		logger:     logger,
		apiURL:     strings.TrimRight(apiURL, "/"),
		httpClient: httpClient,
		cacheTTL:   cacheTTL,
	}
}

func (r *Resolver) GetSandboxOtelConfig(ctx context.Context, authToken string) (*OtelConfig, error) {
	otelConfig, err := r.cache.Get(ctx, authToken)
	if err == nil {
		if otelConfig.Endpoint == "(none)" {
			return nil, nil
		}
		return otelConfig, nil
	}

	config := &OtelConfig{
		Endpoint: "(none)",
	}

	otelConfig, err = r.fetchSandboxOtelConfig(ctx, authToken)
	if err != nil {
		return nil, err
	}

	if otelConfig != nil {
		config = &OtelConfig{
			Endpoint: otelConfig.Endpoint,
			Headers:  otelConfig.Headers,
		}
	}

	if err := r.cache.Set(ctx, authToken, *config, r.cacheTTL); err != nil {
		return nil, err
	}

	if config.Endpoint == "(none)" {
		return nil, nil
	}

	return config, nil
}

func (r *Resolver) fetchSandboxOtelConfig(ctx context.Context, authToken string) (*OtelConfig, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", r.apiURL+"/sandbox/telemetry/otel-config", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create otel config request: %w", err)
	}

	req.Header.Set("sandbox-auth-token", authToken)

	res, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch otel config: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("failed to fetch otel config: status %d", res.StatusCode)
	}

	var config OtelConfig
	if err := json.NewDecoder(res.Body).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode otel config: %w", err)
	}

	return &config, nil
}

// InvalidateCache removes a specific sandbox configuration from the cache.
func (r *Resolver) InvalidateCache(ctx context.Context, sandboxID string) error {
	return r.cache.Delete(ctx, sandboxID)
}
