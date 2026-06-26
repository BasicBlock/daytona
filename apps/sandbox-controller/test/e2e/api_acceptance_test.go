//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestAPIProductAcceptanceE2E(t *testing.T) {
	if os.Getenv("DAYTONA_API_ACCEPTANCE_E2E") != "1" {
		t.Skip("set DAYTONA_API_ACCEPTANCE_E2E=1 to run API product acceptance tests")
	}
	apiBaseURL := strings.TrimRight(requireEnv(t, "DAYTONA_API_BASE_URL"), "/")
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Minute)
	defer cancel()

	k8sClient := e2eClient(t)
	namespace := e2eNamespace(t, k8sClient, ctx)
	name := uniqueName("api-agent")
	forkName := uniqueName("api-fork")
	snapshotName := uniqueName("api-snapshot")

	apiPost(t, ctx, apiBaseURL+"/sandboxes", map[string]any{
		"name": name,
		"spec": map[string]any{
			"image":   "ubuntu:24.04",
			"command": []string{"/bin/sh", "-lc"},
			"args":    []string{"while true; do printf ok >/tmp/daytona-port; sleep 5; done"},
			"ports": []map[string]any{{
				"name": "http",
				"port": 8080,
			}},
			"access": map[string]any{
				"sshEnabled":   true,
				"routeBaseUrl": apiBaseURL,
			},
		},
	}, http.StatusCreated, nil)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		req, _ := http.NewRequestWithContext(cleanupCtx, http.MethodDelete, apiBaseURL+"/sandboxes/"+name, nil)
		_, _ = http.DefaultClient.Do(req)
		req, _ = http.NewRequestWithContext(cleanupCtx, http.MethodDelete, apiBaseURL+"/sandboxes/"+forkName, nil)
		_, _ = http.DefaultClient.Do(req)
	})

	waitForSandboxPhase(ctx, t, apiBaseURL, name, computev1.SandboxPhaseRunning)
	apiPostEventually(t, ctx, apiBaseURL+"/sandboxes/"+name+"/exec", map[string]any{"command": []string{"/bin/sh", "-lc", "echo api-ok"}}, http.StatusOK, nil)
	apiPostEventually(t, ctx, apiBaseURL+"/sandboxes/"+name+"/ports", map[string]any{"name": "http", "port": 8080}, http.StatusOK, nil)
	apiGetEventually(t, ctx, apiBaseURL+"/sandboxes/"+name+"/ssh", http.StatusOK, nil)

	apiPost(t, ctx, apiBaseURL+"/sandboxes/"+name+":snapshot", map[string]any{
		"name":     snapshotName,
		"provider": "GKEPodSnapshot",
		"gke": map[string]any{
			"storageConfigName": envDefault("DAYTONA_ACCEPTANCE_STORAGE_CONFIG", "local-minio"),
			"postCheckpoint":    "resume",
		},
	}, http.StatusCreated, nil)
	waitForSnapshotReady(ctx, t, k8sClient, namespace, snapshotName)

	apiPost(t, ctx, apiBaseURL+"/sandboxes/"+name+":fork", map[string]any{
		"name":         forkName,
		"snapshotName": snapshotName,
	}, http.StatusCreated, nil)
	waitForSandboxPhase(ctx, t, apiBaseURL, forkName, computev1.SandboxPhaseRunning)
	apiPostEventually(t, ctx, apiBaseURL+"/sandboxes/"+forkName+"/exec", map[string]any{"command": []string{"/bin/sh", "-lc", "echo restored"}}, http.StatusOK, nil)
	apiPostEventually(t, ctx, apiBaseURL+"/sandboxes/"+forkName+"/ports", map[string]any{"name": "http", "port": 8080}, http.StatusOK, nil)
	apiGet(t, ctx, apiBaseURL+"/sandboxes", http.StatusOK, nil)

	apiPost(t, ctx, apiBaseURL+"/sandboxes/"+name+":stop", map[string]any{}, http.StatusOK, nil)
	waitForSandboxPhase(ctx, t, apiBaseURL, name, computev1.SandboxPhaseStopped)
	apiPost(t, ctx, apiBaseURL+"/sandboxes/"+name+"/exec", map[string]any{"command": []string{"/bin/sh", "-lc", "echo wake"}}, http.StatusAccepted, nil)
	waitForSandboxPhase(ctx, t, apiBaseURL, name, computev1.SandboxPhaseRunning)

	apiPost(t, ctx, apiBaseURL+"/sandboxes/"+name+":stop", map[string]any{}, http.StatusOK, nil)
	waitForSandboxPhase(ctx, t, apiBaseURL, name, computev1.SandboxPhaseStopped)
	apiPost(t, ctx, apiBaseURL+"/sandboxes/"+name+":start", map[string]any{}, http.StatusOK, nil)
	waitForSandboxPhase(ctx, t, apiBaseURL, name, computev1.SandboxPhaseRunning)

	apiDelete(t, ctx, apiBaseURL+"/sandboxes/"+forkName)
	apiDelete(t, ctx, apiBaseURL+"/sandboxes/"+name)
	waitForOwnedResourcesGone(ctx, t, k8sClient, namespace, name)
	waitForOwnedResourcesGone(ctx, t, k8sClient, namespace, forkName)
}

func waitForSandboxPhase(ctx context.Context, t *testing.T, apiBaseURL string, name string, phase computev1.SandboxPhase) {
	t.Helper()
	waitFor(ctx, t, func() (bool, error) {
		var sandbox computev1.Sandbox
		apiGet(t, ctx, apiBaseURL+"/sandboxes/"+name, http.StatusOK, &sandbox)
		if sandbox.Status.Phase == computev1.SandboxPhaseFailed {
			return false, fmt.Errorf("sandbox %s failed", name)
		}
		return sandbox.Status.Phase == phase, nil
	})
}

func waitForSnapshotReady(ctx context.Context, t *testing.T, k8sClient client.Client, namespace string, name string) {
	t.Helper()
	waitFor(ctx, t, func() (bool, error) {
		var snapshot computev1.SandboxSnapshot
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &snapshot); err != nil {
			return false, err
		}
		if snapshot.Status.Phase == computev1.SandboxSnapshotPhaseFailed {
			return false, fmt.Errorf("snapshot failed: %s", snapshot.Status.Error)
		}
		return snapshot.Status.Phase == computev1.SandboxSnapshotPhaseReady, nil
	})
}

func waitForOwnedResourcesGone(ctx context.Context, t *testing.T, k8sClient client.Client, namespace string, sandboxName string) {
	t.Helper()
	waitFor(ctx, t, func() (bool, error) {
		var pod corev1.Pod
		err := k8sClient.Get(ctx, types.NamespacedName{Name: "sandbox-" + sandboxName, Namespace: namespace}, &pod)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}
		return apierrors.IsNotFound(err), nil
	})
}

func apiGet(t *testing.T, ctx context.Context, url string, wantStatus int, target any) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	apiDo(t, req, wantStatus, target)
}

func apiGetEventually(t *testing.T, ctx context.Context, url string, wantStatus int, target any) {
	t.Helper()
	waitFor(ctx, t, func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return false, err
		}
		return apiDoRetryable(req, wantStatus, target)
	})
}

func apiPost(t *testing.T, ctx context.Context, url string, body any, wantStatus int, target any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	apiDo(t, req, wantStatus, target)
}

func apiPostEventually(t *testing.T, ctx context.Context, url string, body any, wantStatus int, target any) {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	waitFor(ctx, t, func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
		if err != nil {
			return false, err
		}
		req.Header.Set("Content-Type", "application/json")
		return apiDoRetryable(req, wantStatus, target)
	})
}

func apiDelete(t *testing.T, ctx context.Context, url string) {
	t.Helper()
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	apiDo(t, req, http.StatusNoContent, nil)
}

func apiDo(t *testing.T, req *http.Request, wantStatus int, target any) {
	t.Helper()
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != wantStatus {
		t.Fatalf("%s %s expected %d, got %d: %s", req.Method, req.URL, wantStatus, res.StatusCode, string(data))
	}
	if target != nil && len(data) > 0 {
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
}

func apiDoRetryable(req *http.Request, wantStatus int, target any) (bool, error) {
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, nil
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)
	if res.StatusCode != wantStatus {
		switch res.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout, http.StatusNotFound:
			return false, nil
		default:
			return false, fmt.Errorf("%s %s expected %d, got %d: %s", req.Method, req.URL, wantStatus, res.StatusCode, string(data))
		}
	}
	if target != nil && len(data) > 0 {
		if err := json.Unmarshal(data, target); err != nil {
			return false, err
		}
	}
	return true, nil
}
