// Copyright Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build linux

package docker

import "github.com/vishvananda/netlink"

func clearLinkIsolation(link netlink.Link) error {
	return netlink.LinkSetIsolated(link, false)
}
