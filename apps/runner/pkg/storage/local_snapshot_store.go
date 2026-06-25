// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package storage

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type localSnapshotStoreClient struct {
	rootDir  string
	prefix   string
	cacheDir string
}

func NewLocalSnapshotStoreClient(rootDir, prefix, cacheDir string) SnapshotStoreClient {
	return &localSnapshotStoreClient{
		rootDir:  filepath.Clean(rootDir),
		prefix:   strings.Trim(strings.TrimSpace(prefix), "/"),
		cacheDir: filepath.Clean(cacheDir),
	}
}

func (c *localSnapshotStoreClient) Bucket() string {
	return ""
}

func (c *localSnapshotStoreClient) Prefix() string {
	return c.prefix
}

func (c *localSnapshotStoreClient) CacheDir() string {
	return c.cacheDir
}

func (c *localSnapshotStoreClient) Ref(key string) string {
	return BuildLocalFileRef(c.pathForKey(key))
}

func (c *localSnapshotStoreClient) PutObject(ctx context.Context, key string, body io.Reader, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	path := c.pathForKey(key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create local snapshot object directory %s: %w", filepath.Dir(path), err)
	}

	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create local snapshot object %s: %w", path, err)
	}
	defer file.Close()

	if _, err := io.Copy(file, body); err != nil {
		return fmt.Errorf("write local snapshot object %s: %w", path, err)
	}
	return nil
}

func (c *localSnapshotStoreClient) GetObject(ctx context.Context, key string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	path := c.pathForKey(key)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read local snapshot object %s: %w", path, err)
	}
	return data, nil
}

func (c *localSnapshotStoreClient) DownloadObject(ctx context.Context, key string, localPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	src := c.pathForKey(key)
	dst := filepath.Clean(localPath)
	if src == dst {
		return nil
	}

	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open local snapshot object %s: %w", src, err)
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create local snapshot download directory %s: %w", filepath.Dir(dst), err)
	}
	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create local snapshot download %s: %w", dst, err)
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return fmt.Errorf("copy local snapshot object %s to %s: %w", src, dst, err)
	}
	return nil
}

func (c *localSnapshotStoreClient) ObjectExists(ctx context.Context, key string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}

	_, err := os.Stat(c.pathForKey(key))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (c *localSnapshotStoreClient) pathForKey(key string) string {
	key = strings.TrimSpace(key)
	if IsLocalFileRef(key) {
		parsed, err := ParseLocalFileRef(key)
		if err == nil {
			return parsed.Path
		}
	}
	if filepath.IsAbs(key) {
		return filepath.Clean(key)
	}

	cleanKey := strings.TrimPrefix(filepath.ToSlash(filepath.Clean(key)), "/")
	if c.prefix != "" && cleanKey != c.prefix && !strings.HasPrefix(cleanKey, c.prefix+"/") {
		cleanKey = c.prefix + "/" + cleanKey
	}
	return filepath.Join(c.rootDir, filepath.FromSlash(cleanKey))
}
