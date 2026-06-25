// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build linux

package port

import (
	"strconv"

	"github.com/cakturk/go-netstat/netstat"
)

func listeningTCPPorts() (map[string]bool, error) {
	tabs, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
		return s.State == netstat.Listen
	})
	if err != nil {
		return nil, err
	}

	ports := map[string]bool{}
	for _, e := range tabs {
		ports[strconv.Itoa(int(e.LocalAddr.Port))] = true
	}

	return ports, nil
}
