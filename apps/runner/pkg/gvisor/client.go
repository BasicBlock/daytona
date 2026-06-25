// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package gvisor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"

	"github.com/daytonaio/runner/cmd/runner/config"
)

type Client struct {
	path       string
	globalArgs []string
	logger     *slog.Logger
}

func NewClientFromConfig(logger *slog.Logger) (*Client, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	globalArgs := make([]string, 0)
	if cfg.RunscRoot != "" {
		globalArgs = append(globalArgs, "--root="+cfg.RunscRoot)
	}
	if cfg.RunscConfigFile != "" {
		globalArgs = append(globalArgs, "--config="+cfg.RunscConfigFile)
	}
	if cfg.RunscExtraArgs != "" {
		globalArgs = append(globalArgs, strings.Fields(cfg.RunscExtraArgs)...)
	}

	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		path:       cfg.RunscPath,
		globalArgs: globalArgs,
		logger:     logger.With(slog.String("component", "gvisor-client")),
	}, nil
}

func (c *Client) Version(ctx context.Context) string {
	output, err := c.output(ctx, "--version")
	if err != nil {
		c.logger.WarnContext(ctx, "Failed to get runsc version", "error", err)
		return ""
	}
	return strings.TrimSpace(output)
}

func (c *Client) CPUFeatures(ctx context.Context) []string {
	output, err := c.output(ctx, append(c.globalArgs, "cpu-features")...)
	if err != nil {
		c.logger.WarnContext(ctx, "Failed to get runsc CPU features", "error", err)
		return nil
	}

	features := make([]string, 0)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			features = append(features, line)
		}
	}
	return features
}

func (c *Client) Pause(ctx context.Context, containerID string) error {
	return c.run(ctx, "pause", containerID)
}

func (c *Client) Resume(ctx context.Context, containerID string) error {
	return c.run(ctx, "resume", containerID)
}

func (c *Client) FSCheckpoint(ctx context.Context, containerID string, imagePath string) error {
	return c.run(ctx, "fscheckpoint", "--image-path="+imagePath, "--leave-running", "--path=/", containerID)
}

func (c *Client) Checkpoint(ctx context.Context, containerID string, imagePath string) error {
	return c.run(ctx, "checkpoint", "--image-path="+imagePath, "--leave-running", "--compression=none", containerID)
}

func (c *Client) run(ctx context.Context, args ...string) error {
	_, err := c.output(ctx, append(c.globalArgs, args...)...)
	return err
}

func (c *Client) output(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, c.path, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("runsc %s failed: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}

	if stderr.Len() > 0 {
		c.logger.DebugContext(ctx, "runsc stderr", "args", strings.Join(args, " "), "stderr", strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}
