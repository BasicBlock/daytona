// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package netrules

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-iptables/iptables"
)

// NetRulesManager provides thread-safe operations for managing network rules
type NetRulesManager struct {
	log        *slog.Logger
	ipt        *iptables.IPTables
	mu         sync.Mutex
	persistent bool
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewNetRulesManager creates a new instance of NetRulesManager
func NewNetRulesManager(logger *slog.Logger, persistent bool) (*NetRulesManager, error) {
	if logger == nil {
		return nil, errors.New("logger can't be nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	manager := &NetRulesManager{
		log:        logger.With(slog.String("component", "netrules_manager")),
		persistent: persistent,
		ctx:        ctx,
		cancel:     cancel,
	}

	ipt, err := iptables.NewWithProtocol(iptables.ProtocolIPv4)
	if err != nil {
		if runtime.GOOS != "linux" {
			manager.log.Warn("iptables unavailable; network rules are disabled on this platform", "error", err)
			return manager, nil
		}

		cancel()
		return nil, err
	}

	manager.ipt = ipt

	return manager, nil
}

func (manager *NetRulesManager) Start() error {
	if manager.disabled() {
		return nil
	}

	// Start periodic reconciliation
	if manager.persistent {
		go manager.persistRulesLoop()
	}

	return nil
}

// Stop gracefully stops the NetRulesManager
func (manager *NetRulesManager) Stop() {
	if manager.cancel != nil {
		manager.cancel()
	}
}

// saveIptablesRules saves the current iptables rules to make them persistent
func (manager *NetRulesManager) saveIptablesRules() error {
	if manager.disabled() {
		return nil
	}

	if manager.persistent {
		cmd := exec.Command("sh", "-c", "iptables-save > /etc/iptables/rules.v4")
		return cmd.Run()
	}
	return nil
}

// ListDaytonaRules returns all DOCKER-USER rules that jump to Daytona chains
func (manager *NetRulesManager) ListDaytonaRules(table string, chain string) ([]string, error) {
	if manager.disabled() {
		return nil, nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	rules, err := manager.ipt.List(table, chain)
	if err != nil {
		return nil, err
	}

	var daytonaRules []string
	for _, rule := range rules {
		if strings.Contains(rule, ChainPrefix) {
			daytonaRules = append(daytonaRules, rule)
		}
	}

	return daytonaRules, nil
}

// DeleteChainRule deletes a specific rule from a specific chain
func (manager *NetRulesManager) DeleteChainRule(table string, chain string, rule string) error {
	if manager.disabled() {
		return nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	args, err := ParseRuleArguments(rule)
	if err != nil {
		return err
	}

	return manager.ipt.Delete(table, chain, args...)
}

// ListDaytonaChains returns all chains that start with DAYTONA-SB-
func (manager *NetRulesManager) ListDaytonaChains(table string) ([]string, error) {
	if manager.disabled() {
		return nil, nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	chains, err := manager.ipt.ListChains(table)
	if err != nil {
		return nil, err
	}

	var daytonaChains []string
	for _, chain := range chains {
		if strings.HasPrefix(chain, ChainPrefix) {
			daytonaChains = append(daytonaChains, chain)
		}
	}

	return daytonaChains, nil
}

// ClearAndDeleteChain deletes a specific table chain
func (manager *NetRulesManager) ClearAndDeleteChain(table string, name string) error {
	if manager.disabled() {
		return nil
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()

	return manager.ipt.ClearAndDeleteChain(table, name)
}

func (manager *NetRulesManager) disabled() bool {
	return manager == nil || manager.ipt == nil
}

// persistRulesLoop persists the iptables rules
func (manager *NetRulesManager) persistRulesLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	manager.log.Info("Starting iptables persistence loop")

	for {
		select {
		case <-manager.ctx.Done():
			manager.log.Info("Stopping iptables persistence loop")
			return
		case <-ticker.C:
			if err := manager.saveIptablesRules(); err != nil {
				manager.log.Error("Failed to save iptables rules", "error", err)
			}
		}
	}
}
