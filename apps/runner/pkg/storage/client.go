// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package storage

import (
	"context"
	"io"
)

// ObjectStorageClient defines the interface for object storage operations
type ObjectStorageClient interface {
	GetObject(ctx context.Context, storagePrefix, hash string) ([]byte, error)
}

type SnapshotStoreClient interface {
	Bucket() string
	Prefix() string
	CacheDir() string
	Ref(key string) string
	// PutObject writes relative to the configured snapshot prefix. GetObject,
	// DownloadObject, and ObjectExists read exact bucket object keys, usually
	// parsed from a persisted gcs:// ref or manifest.
	PutObject(ctx context.Context, key string, body io.Reader, contentType string) error
	GetObject(ctx context.Context, key string) ([]byte, error)
	DownloadObject(ctx context.Context, key string, localPath string) error
	ObjectExists(ctx context.Context, key string) (bool, error)
}
