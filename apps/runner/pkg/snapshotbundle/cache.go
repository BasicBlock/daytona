// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package snapshotbundle

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/daytonaio/runner/pkg/storage"
)

type CachedBundle struct {
	Manifest              Manifest
	ManifestPath          string
	CheckpointArchivePath string
	FilesystemArchivePath string
	CheckpointDir         string
	FilesystemDir         string
}

func ManifestFromBytes(data []byte) (Manifest, error) {
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("unmarshal snapshot manifest: %w", err)
	}
	if manifest.Version != Version {
		return Manifest{}, fmt.Errorf("unsupported snapshot manifest version %q", manifest.Version)
	}
	if manifest.Hash == "" {
		return Manifest{}, fmt.Errorf("snapshot manifest missing hash")
	}
	if manifest.Objects.CheckpointKey == "" {
		return Manifest{}, fmt.Errorf("snapshot manifest missing checkpoint object")
	}
	if manifest.Objects.FilesystemStateKey == "" {
		return Manifest{}, fmt.Errorf("snapshot manifest missing filesystem object")
	}
	return manifest, nil
}

func MarshalManifest(manifest Manifest) ([]byte, error) {
	return json.MarshalIndent(manifest, "", "  ")
}

func CacheFromRef(ctx context.Context, store storage.SnapshotStoreClient, ref string) (*CachedBundle, error) {
	manifestKey, err := storage.SnapshotObjectKeyFromRef(ref)
	if err != nil {
		return nil, err
	}
	if storage.IsGCSRef(ref) {
		gcsRef, err := storage.ParseGCSRef(ref)
		if err != nil {
			return nil, err
		}
		if gcsRef.Bucket != store.Bucket() {
			return nil, fmt.Errorf("snapshot ref bucket %q does not match configured bucket %q", gcsRef.Bucket, store.Bucket())
		}
	}

	manifestBytes, err := store.GetObject(ctx, manifestKey)
	if err != nil {
		return nil, err
	}
	manifest, err := ManifestFromBytes(manifestBytes)
	if err != nil {
		return nil, err
	}
	if manifest.Ref == "" {
		manifest.Ref = ref
	}

	baseDir := filepath.Join(store.CacheDir(), "gvisor", manifest.Hash)
	checkpointArchivePath := filepath.Join(baseDir, "checkpoint.tar")
	filesystemArchivePath := filepath.Join(baseDir, "filesystem.tar")
	checkpointDir := filepath.Join(baseDir, "checkpoint")
	filesystemDir := filepath.Join(baseDir, "filesystem")
	manifestPath := filepath.Join(baseDir, ManifestFileName)

	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("create snapshot cache directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, manifestBytes, 0644); err != nil {
		return nil, fmt.Errorf("write cached snapshot manifest: %w", err)
	}

	if _, err := os.Stat(checkpointArchivePath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := store.DownloadObject(ctx, manifest.Objects.CheckpointKey, checkpointArchivePath); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filesystemArchivePath); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := store.DownloadObject(ctx, manifest.Objects.FilesystemStateKey, filesystemArchivePath); err != nil {
			return nil, err
		}
	}

	if _, err := os.Stat(checkpointDir); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := UntarDirectory(checkpointArchivePath, checkpointDir); err != nil {
			return nil, err
		}
	}
	if _, err := os.Stat(filesystemDir); err != nil {
		if !os.IsNotExist(err) {
			return nil, err
		}
		if err := UntarDirectory(filesystemArchivePath, filesystemDir); err != nil {
			return nil, err
		}
	}

	return &CachedBundle{
		Manifest:              manifest,
		ManifestPath:          manifestPath,
		CheckpointArchivePath: checkpointArchivePath,
		FilesystemArchivePath: filesystemArchivePath,
		CheckpointDir:         checkpointDir,
		FilesystemDir:         filesystemDir,
	}, nil
}

func CacheFromGCS(ctx context.Context, store storage.SnapshotStoreClient, ref string) (*CachedBundle, error) {
	return CacheFromRef(ctx, store, ref)
}

func UploadManifest(ctx context.Context, store storage.SnapshotStoreClient, key string, manifest Manifest) ([]byte, error) {
	data, err := MarshalManifest(manifest)
	if err != nil {
		return nil, err
	}
	if err := store.PutObject(ctx, key, bytes.NewReader(data), MediaType); err != nil {
		return nil, err
	}
	return data, nil
}
