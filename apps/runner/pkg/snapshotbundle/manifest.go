// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package snapshotbundle

import (
	"runtime"
	"time"
)

const (
	ManifestFileName = "manifest.json"
	MediaType        = "application/vnd.daytona.gvisor-snapshot.manifest.v1+json"
	Version          = "gvisor-snapshot/v1"
)

type Manifest struct {
	Version         string            `json:"version"`
	Name            string            `json:"name"`
	Ref             string            `json:"ref"`
	Hash            string            `json:"hash"`
	CreatedAt       time.Time         `json:"createdAt"`
	BaseImageRef    string            `json:"baseImageRef"`
	BaseImageDigest string            `json:"baseImageDigest,omitempty"`
	Runtime         RuntimeMetadata   `json:"runtime"`
	Objects         SnapshotObjects   `json:"objects"`
	Config          RuntimeConfig     `json:"config"`
	Compatibility   CompatibilityData `json:"compatibility"`
	SizeBytes       int64             `json:"sizeBytes"`
	Labels          map[string]string `json:"labels,omitempty"`
}

type RuntimeMetadata struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type SnapshotObjects struct {
	CheckpointKey      string `json:"checkpointKey"`
	FilesystemStateKey string `json:"filesystemStateKey"`
}

type RuntimeConfig struct {
	Entrypoint []string          `json:"entrypoint,omitempty"`
	Cmd        []string          `json:"cmd,omitempty"`
	Env        map[string]string `json:"env,omitempty"`
	WorkDir    string            `json:"workDir,omitempty"`
	OsUser     string            `json:"osUser,omitempty"`
}

type CompatibilityData struct {
	Architecture string   `json:"architecture"`
	OS           string   `json:"os"`
	CPUFeatures  []string `json:"cpuFeatures,omitempty"`
}

func New(name string) Manifest {
	return Manifest{
		Version:   Version,
		Name:      name,
		CreatedAt: time.Now().UTC(),
		Runtime: RuntimeMetadata{
			Name: "runsc",
		},
		Compatibility: CompatibilityData{
			Architecture: runtime.GOARCH,
			OS:           "linux",
		},
	}
}
