// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package proxy

import (
	"fmt"
	"strings"
)

func (p *Proxy) getCookieDomain(host string) string {
	if p.cookieDomain != nil {
		return *p.cookieDomain
	}
	return GetCookieDomainFromHost(host)
}

func GetCookieDomainFromHost(host string) string {
	host = strings.Split(host, ":")[0]
	return fmt.Sprintf(".%s", host)
}
