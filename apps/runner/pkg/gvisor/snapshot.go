// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package gvisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/daytonaio/runner/pkg/snapshotbundle"
	"github.com/daytonaio/runner/pkg/storage"
)

type SnapshotOptions struct {
	SandboxID       string
	Name            string
	BaseImageRef    string
	BaseImageDigest string
	RuntimeConfig   snapshotbundle.RuntimeConfig
	Labels          map[string]string
}

func (c *Client) CreateSnapshot(ctx context.Context, store storage.SnapshotStoreClient, opts SnapshotOptions) (*snapshotbundle.Manifest, error) {
	if opts.SandboxID == "" {
		return nil, fmt.Errorf("sandbox id is required")
	}
	if opts.Name == "" {
		return nil, fmt.Errorf("snapshot name is required")
	}
	if opts.BaseImageRef == "" {
		return nil, fmt.Errorf("base image ref is required")
	}

	stagingDir := filepath.Join(store.CacheDir(), "staging", safePathPart(opts.Name)+"-"+time.Now().UTC().Format("20060102150405.000000000"))
	checkpointDir := filepath.Join(stagingDir, "checkpoint")
	filesystemDir := filepath.Join(stagingDir, "filesystem")
	artifactDir := filepath.Join(stagingDir, "artifacts")

	if err := os.MkdirAll(checkpointDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filesystemDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		return nil, err
	}
	defer os.RemoveAll(stagingDir)

	paused := false
	if err := c.Pause(ctx, opts.SandboxID); err != nil {
		return nil, fmt.Errorf("pause sandbox before gVisor snapshot: %w", err)
	}
	paused = true
	defer func() {
		if paused {
			if err := c.Resume(context.Background(), opts.SandboxID); err != nil {
				c.logger.ErrorContext(context.Background(), "Failed to resume sandbox after gVisor snapshot error", "sandboxId", opts.SandboxID, "error", err)
			}
		}
	}()

	if err := c.FSCheckpoint(ctx, opts.SandboxID, filesystemDir); err != nil {
		return nil, fmt.Errorf("create gVisor filesystem checkpoint: %w", err)
	}
	if err := c.Checkpoint(ctx, opts.SandboxID, checkpointDir); err != nil {
		return nil, fmt.Errorf("create gVisor memory checkpoint: %w", err)
	}
	checkpointTar := filepath.Join(artifactDir, "checkpoint.tar")
	filesystemTar := filepath.Join(artifactDir, "filesystem.tar")
	checkpointSize, err := snapshotbundle.TarDirectory(checkpointDir, checkpointTar)
	if err != nil {
		return nil, err
	}
	filesystemSize, err := snapshotbundle.TarDirectory(filesystemDir, filesystemTar)
	if err != nil {
		return nil, err
	}
	hash, err := snapshotbundle.HashFiles(checkpointTar, filesystemTar)
	if err != nil {
		return nil, err
	}

	objectBase := filepath.ToSlash(filepath.Join("gvisor", safePathPart(opts.Name), hash))
	checkpointKey := filepath.ToSlash(filepath.Join(objectBase, "checkpoint.tar"))
	filesystemKey := filepath.ToSlash(filepath.Join(objectBase, "filesystem.tar"))
	manifestKey := filepath.ToSlash(filepath.Join(objectBase, snapshotbundle.ManifestFileName))

	checkpointFullKey, err := storage.SnapshotObjectKeyFromRef(store.Ref(checkpointKey))
	if err != nil {
		return nil, err
	}
	filesystemFullKey, err := storage.SnapshotObjectKeyFromRef(store.Ref(filesystemKey))
	if err != nil {
		return nil, err
	}

	if err := uploadFile(ctx, store, checkpointKey, checkpointTar); err != nil {
		return nil, err
	}
	if err := uploadFile(ctx, store, filesystemKey, filesystemTar); err != nil {
		return nil, err
	}

	manifest := snapshotbundle.New(opts.Name)
	manifest.Ref = store.Ref(manifestKey)
	manifest.Hash = hash
	manifest.BaseImageRef = opts.BaseImageRef
	manifest.BaseImageDigest = opts.BaseImageDigest
	manifest.Runtime.Version = c.Version(ctx)
	manifest.Objects.CheckpointKey = checkpointFullKey
	manifest.Objects.FilesystemStateKey = filesystemFullKey
	manifest.Config = opts.RuntimeConfig
	manifest.Compatibility.CPUFeatures = c.CPUFeatures(ctx)
	manifest.SizeBytes = checkpointSize + filesystemSize
	manifest.Labels = opts.Labels

	if _, err := snapshotbundle.UploadManifest(ctx, store, manifestKey, manifest); err != nil {
		return nil, err
	}

	return &manifest, nil
}

func uploadFile(ctx context.Context, store storage.SnapshotStoreClient, key string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return store.PutObject(ctx, key, file, snapshotbundle.TarContentType)
}

var unsafePathPartChars = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func safePathPart(value string) string {
	value = strings.Trim(unsafePathPartChars.ReplaceAllString(value, "-"), "-.")
	if value == "" {
		return "snapshot"
	}
	return value
}
