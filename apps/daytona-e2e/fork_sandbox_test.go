// Copyright Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

//go:build e2e

package e2e_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForkSandboxPreservesFilesystemState(t *testing.T) {
	cfg := LoadConfig(t)
	client := NewAPIClient(cfg)
	runID := testRunID()

	markerPath := "/tmp/e2e-fork-marker.txt"
	markerContent := fmt.Sprintf("e2e-fork-content-%d", time.Now().UnixNano())
	sourceName := fmt.Sprintf("e2e-fork-src-%s", runID[4:])
	forkName := fmt.Sprintf("e2e-fork-dst-%s", runID[4:])

	createReq := map[string]interface{}{
		"name":   sourceName,
		"labels": sandboxLabels(runID),
	}
	if cfg.Snapshot != "" {
		createReq["snapshot"] = cfg.Snapshot
	}

	srcSandbox := client.CreateSandbox(t, createReq)
	srcSandboxID, _ := srcSandbox["id"].(string)
	require.NotEmpty(t, srcSandboxID, "source sandbox must have id")

	srcStarted := client.PollSandboxState(t, srcSandboxID, "started", cfg.PollTimeout, cfg.PollInterval)
	srcToolboxURL, _ := srcStarted["toolboxProxyUrl"].(string)
	require.NotEmpty(t, srcToolboxURL, "source sandbox must expose toolboxProxyUrl")

	httpCli := &http.Client{Timeout: 60 * time.Second}
	srcBaseURL := strings.TrimRight(srcToolboxURL, "/") + "/" + srcSandboxID
	uploadFile(t, httpCli, cfg, srcBaseURL, markerPath, markerContent)

	resp, body := client.DoRequest(t, http.MethodPost, "/sandbox/"+srcSandboxID+"/fork", map[string]interface{}{
		"name": forkName,
	})
	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
	default:
		t.Fatalf("POST /sandbox/%s/fork returned %d: %s", srcSandboxID, resp.StatusCode, string(body))
	}

	var forked map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &forked), "failed to parse fork response")
	forkID, _ := forked["id"].(string)
	require.NotEmpty(t, forkID, "fork response must include id")
	t.Cleanup(func() {
		client.DeleteSandbox(t, forkID)
	})

	client.PollSandboxState(t, srcSandboxID, "started", cfg.PollTimeout, cfg.PollInterval)
	forkStarted := client.PollSandboxState(t, forkID, "started", cfg.PollTimeout, cfg.PollInterval)
	forkToolboxURL, _ := forkStarted["toolboxProxyUrl"].(string)
	require.NotEmpty(t, forkToolboxURL, "forked sandbox must expose toolboxProxyUrl")

	forkBaseURL := strings.TrimRight(forkToolboxURL, "/") + "/" + forkID
	downloaded := downloadFile(t, httpCli, cfg, forkBaseURL, markerPath)
	assert.Contains(t, downloaded, markerContent, "fork must preserve filesystem state from source sandbox")
}
