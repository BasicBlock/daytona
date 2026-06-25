// Copyright Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build !linux

package docker

import "github.com/vishvananda/netlink"

func clearLinkIsolation(_ netlink.Link) error {
	return nil
}
