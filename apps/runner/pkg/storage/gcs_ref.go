// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package storage

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

const (
	GCSRefScheme       = "gcs"
	LocalFileRefScheme = "file"
)

type GCSRef struct {
	Bucket string
	Key    string
}

type LocalFileRef struct {
	Path string
}

func IsSnapshotStoreRef(ref string) bool {
	return IsGCSRef(ref) || IsLocalFileRef(ref)
}

func IsGCSRef(ref string) bool {
	return strings.HasPrefix(ref, GCSRefScheme+"://")
}

func IsLocalFileRef(ref string) bool {
	return strings.HasPrefix(ref, LocalFileRefScheme+"://")
}

func ParseGCSRef(ref string) (GCSRef, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return GCSRef{}, err
	}
	if u.Scheme != GCSRefScheme || u.Host == "" {
		return GCSRef{}, fmt.Errorf("invalid GCS ref: %s", ref)
	}
	key := strings.TrimPrefix(u.Path, "/")
	if key == "" {
		return GCSRef{}, fmt.Errorf("invalid GCS ref without object key: %s", ref)
	}
	return GCSRef{Bucket: u.Host, Key: key}, nil
}

func ParseLocalFileRef(ref string) (LocalFileRef, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return LocalFileRef{}, err
	}
	if u.Scheme != LocalFileRefScheme || (u.Host != "" && u.Host != "localhost") {
		return LocalFileRef{}, fmt.Errorf("invalid local snapshot ref: %s", ref)
	}
	path := filepath.Clean(u.Path)
	if path == "." || path == "" {
		return LocalFileRef{}, fmt.Errorf("invalid local snapshot ref without path: %s", ref)
	}
	if !filepath.IsAbs(path) {
		return LocalFileRef{}, fmt.Errorf("local snapshot ref path must be absolute: %s", ref)
	}
	return LocalFileRef{Path: path}, nil
}

func BuildGCSRef(bucket, key string) string {
	return fmt.Sprintf("%s://%s/%s", GCSRefScheme, bucket, strings.TrimPrefix(key, "/"))
}

func BuildLocalFileRef(path string) string {
	return (&url.URL{Scheme: LocalFileRefScheme, Path: filepath.Clean(path)}).String()
}

func SnapshotObjectKeyFromRef(ref string) (string, error) {
	if IsGCSRef(ref) {
		parsed, err := ParseGCSRef(ref)
		if err != nil {
			return "", err
		}
		return parsed.Key, nil
	}
	if IsLocalFileRef(ref) {
		parsed, err := ParseLocalFileRef(ref)
		if err != nil {
			return "", err
		}
		return parsed.Path, nil
	}
	return "", fmt.Errorf("unsupported snapshot artifact ref: %s", ref)
}
