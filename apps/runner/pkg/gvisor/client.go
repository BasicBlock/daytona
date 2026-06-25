// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package gvisor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
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

func (c *Client) Run(ctx context.Context, containerID string, bundleDir string) error {
	return c.runDetached(ctx, "run", "--bundle="+bundleDir, "--detach", containerID)
}

func (c *Client) Restore(ctx context.Context, containerID string, bundleDir string, checkpointDir string, filesystemDir string) error {
	return c.runDetached(ctx,
		"restore",
		"--bundle="+bundleDir,
		"--image-path="+checkpointDir,
		"--fs-restore-image-path="+filesystemDir,
		"--detach",
		containerID,
	)
}

func (c *Client) Kill(ctx context.Context, containerID string, signal string) error {
	if signal == "" {
		signal = "TERM"
	}
	return c.run(ctx, "kill", containerID, signal)
}

func (c *Client) Delete(ctx context.Context, containerID string) error {
	return c.run(ctx, "delete", containerID)
}

type State struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Pid    int    `json:"pid"`
	Bundle string `json:"bundle"`
}

func (c *Client) State(ctx context.Context, containerID string) (*State, error) {
	output, err := c.output(ctx, append(c.globalArgs, "state", containerID)...)
	if err != nil {
		return nil, err
	}
	var state State
	if err := json.Unmarshal([]byte(output), &state); err != nil {
		return nil, fmt.Errorf("unmarshal runsc state for %s: %w", containerID, err)
	}
	return &state, nil
}

func (c *Client) run(ctx context.Context, args ...string) error {
	_, err := c.output(ctx, append(c.globalArgs, args...)...)
	return err
}

func (c *Client) runDetached(ctx context.Context, args ...string) error {
	fullArgs := append(c.globalArgs, args...)
	cmd := exec.CommandContext(ctx, c.path, fullArgs...)

	stdio, err := os.CreateTemp("", "daytona-runsc-stdio-*.log")
	if err != nil {
		return fmt.Errorf("create runsc stdio log: %w", err)
	}
	stdioPath := stdio.Name()
	defer os.Remove(stdioPath)
	defer stdio.Close()

	cmd.Stdout = stdio
	cmd.Stderr = stdio

	if err := cmd.Run(); err != nil {
		_, _ = stdio.Seek(0, 0)
		data, _ := os.ReadFile(stdioPath)
		return fmt.Errorf("runsc %s failed: %w: %s", strings.Join(fullArgs, " "), err, strings.TrimSpace(string(data)))
	}
	return nil
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
