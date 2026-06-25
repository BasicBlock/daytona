// Copyright 2025 Daytona Platforms Inc.
// SPDX-License-Identifier: AGPL-3.0

package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/daytonaio/runner/cmd/runner/config"
)

var runnerID string

type runnerIdentity struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Target string `json:"target"`
}

type createRunnerResponse struct {
	ID string `json:"id"`
}

func GetRunnerID(ctx context.Context) (string, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return "", err
	}

	if cfg.RunnerId != "" {
		return cfg.RunnerId, nil
	}
	if runnerID != "" {
		return runnerID, nil
	}

	id, err := lookupRunnerID(ctx, cfg)
	if err == nil && id != "" {
		runnerID = id
		return runnerID, nil
	}

	id, err = createRunner(ctx, cfg)
	if err != nil {
		return "", err
	}
	runnerID = id
	return runnerID, nil
}

func lookupRunnerID(ctx context.Context, cfg *config.Config) (string, error) {
	endpoint := strings.TrimRight(cfg.DaytonaApiUrl, "/") + "/runners"
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("target", cfg.RunnerTarget)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	addRunnerHeaders(req, cfg)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("list runners returned %s: %s", resp.Status, string(body))
	}

	var runners []runnerIdentity
	if err := json.Unmarshal(body, &runners); err != nil {
		return "", err
	}

	for _, runner := range runners {
		if runner.Name == cfg.RunnerName && (runner.Target == "" || runner.Target == cfg.RunnerTarget) {
			return runner.ID, nil
		}
	}

	return "", nil
}

func createRunner(ctx context.Context, cfg *config.Config) (string, error) {
	payload, err := json.Marshal(map[string]any{
		"target": cfg.RunnerTarget,
		"name":   cfg.RunnerName,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		strings.TrimRight(cfg.DaytonaApiUrl, "/")+"/runners",
		bytes.NewReader(payload),
	)
	if err != nil {
		return "", err
	}
	addRunnerHeaders(req, cfg)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusConflict {
		deadline, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		return lookupRunnerID(deadline, cfg)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return "", fmt.Errorf("create runner returned %s: %s", resp.Status, string(body))
	}

	var created createRunnerResponse
	if err := json.Unmarshal(body, &created); err != nil {
		return "", err
	}
	if created.ID == "" {
		return "", fmt.Errorf("create runner response did not include id")
	}

	return created.ID, nil
}

func addRunnerHeaders(req *http.Request, cfg *config.Config) {
	req.Header.Set(DaytonaSourceHeader, "runner")
}
