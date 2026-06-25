// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	gcs "cloud.google.com/go/storage"
	"github.com/daytonaio/runner/cmd/runner/config"
	"google.golang.org/api/iterator"
)

type gcsSnapshotStoreClient struct {
	client   *gcs.Client
	bucket   string
	prefix   string
	cacheDir string
}

var snapshotStoreInstance SnapshotStoreClient

func GetSnapshotStoreClient(ctx context.Context) (SnapshotStoreClient, error) {
	if snapshotStoreInstance != nil {
		return snapshotStoreInstance, nil
	}

	runnerConfig, err := config.GetConfig()
	if err != nil {
		return nil, err
	}
	if runnerConfig.SnapshotGCSBucket == "" {
		snapshotStoreInstance = NewLocalSnapshotStoreClient(
			filepath.Join(runnerConfig.SnapshotCacheDir, "objects"),
			runnerConfig.SnapshotGCSPrefix,
			runnerConfig.SnapshotCacheDir,
		)
		return snapshotStoreInstance, nil
	}

	client, err := gcs.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create GCS client: %w", err)
	}

	snapshotStoreInstance = &gcsSnapshotStoreClient{
		client:   client,
		bucket:   runnerConfig.SnapshotGCSBucket,
		prefix:   strings.Trim(strings.TrimSpace(runnerConfig.SnapshotGCSPrefix), "/"),
		cacheDir: runnerConfig.SnapshotCacheDir,
	}

	return snapshotStoreInstance, nil
}

func (c *gcsSnapshotStoreClient) Bucket() string {
	return c.bucket
}

func (c *gcsSnapshotStoreClient) Prefix() string {
	return c.prefix
}

func (c *gcsSnapshotStoreClient) CacheDir() string {
	return c.cacheDir
}

func (c *gcsSnapshotStoreClient) Ref(key string) string {
	return BuildGCSRef(c.bucket, c.fullKey(key))
}

func (c *gcsSnapshotStoreClient) PutObject(ctx context.Context, key string, body io.Reader, contentType string) error {
	writer := c.client.Bucket(c.bucket).Object(c.fullKey(key)).NewWriter(ctx)
	if contentType != "" {
		writer.ContentType = contentType
	}
	if _, err := io.Copy(writer, body); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write GCS object %s: %w", key, err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close GCS object %s: %w", key, err)
	}
	return nil
}

func (c *gcsSnapshotStoreClient) GetObject(ctx context.Context, key string) ([]byte, error) {
	reader, err := c.client.Bucket(c.bucket).Object(NormalizeSnapshotObjectKey(key)).NewReader(ctx)
	if err != nil {
		return nil, fmt.Errorf("open GCS object %s: %w", key, err)
	}
	defer reader.Close()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read GCS object %s: %w", key, err)
	}
	return data, nil
}

func (c *gcsSnapshotStoreClient) DownloadObject(ctx context.Context, key string, localPath string) error {
	reader, err := c.client.Bucket(c.bucket).Object(NormalizeSnapshotObjectKey(key)).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("open GCS object %s: %w", key, err)
	}
	defer reader.Close()

	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("create local object directory %s: %w", filepath.Dir(localPath), err)
	}
	file, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create local object %s: %w", localPath, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("download GCS object %s: %w", key, err)
	}
	return nil
}

func (c *gcsSnapshotStoreClient) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := c.client.Bucket(c.bucket).Object(NormalizeSnapshotObjectKey(key)).Attrs(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return false, nil
	}
	return false, err
}

func (c *gcsSnapshotStoreClient) fullKey(key string) string {
	key = strings.TrimPrefix(key, "/")
	if c.prefix == "" {
		return key
	}
	if strings.HasPrefix(key, c.prefix+"/") {
		return key
	}
	return c.prefix + "/" + key
}

func NormalizeSnapshotObjectKey(key string) string {
	return strings.TrimPrefix(key, "/")
}

func ListSnapshotObjectKeys(ctx context.Context, client *gcs.Client, bucket string, prefix string) ([]string, error) {
	var keys []string
	it := client.Bucket(bucket).Objects(ctx, &gcs.Query{Prefix: prefix})
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		keys = append(keys, attrs.Name)
	}
	return keys, nil
}
