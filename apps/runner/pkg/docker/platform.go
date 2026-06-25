// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package docker

import (
	"runtime"
	"strings"

	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

func sandboxPlatform() *v1.Platform {
	return &v1.Platform{
		Architecture: sandboxArchitecture(),
		OS:           "linux",
	}
}

func sandboxPlatformString() string {
	return "linux/" + sandboxArchitecture()
}

func sandboxArchitecture() string {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64"
	case "arm64":
		return "arm64"
	default:
		return runtime.GOARCH
	}
}

func isRuntimeCompatibleArchitecture(arch string) bool {
	switch strings.ToLower(arch) {
	case "":
		return false
	case sandboxArchitecture():
		return true
	case "x86_64":
		return sandboxArchitecture() == "amd64"
	case "aarch64":
		return sandboxArchitecture() == "arm64"
	default:
		return false
	}
}
