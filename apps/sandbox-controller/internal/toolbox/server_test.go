package toolbox

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestReadyRequiresSandboxIdentity(t *testing.T) {
	t.Setenv("DAYTONA_SANDBOX_NAME", "")
	server := New()

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", res.Code)
	}
}

func TestIdentityReloadsEnvironment(t *testing.T) {
	t.Setenv("DAYTONA_SANDBOX_NAME", "agent")
	t.Setenv("DAYTONA_WORKLOAD_CONTAINER", "workload")
	t.Setenv("DAYTONA_SSH_ENABLED", "true")

	server := New()
	server.now = func() time.Time { return time.Unix(10, 0) }

	req := httptest.NewRequest(http.MethodGet, "/identity", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var identity Identity
	if err := json.NewDecoder(res.Body).Decode(&identity); err != nil {
		t.Fatal(err)
	}
	if identity.SandboxName != "agent" || identity.WorkloadContainer != "workload" || !identity.SSHEnabled {
		t.Fatalf("unexpected identity: %#v", identity)
	}
}

func TestExecRunsThroughExecutor(t *testing.T) {
	t.Setenv("DAYTONA_SANDBOX_NAME", "agent")
	t.Setenv("DAYTONA_WORKLOAD_CONTAINER", "workload")
	server := New()
	server.executor = fakeExecutor{}

	req := httptest.NewRequest(http.MethodPost, "/exec", strings.NewReader(`{"command":["/bin/echo","ok"]}`))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var execResult ExecResponse
	if err := json.NewDecoder(res.Body).Decode(&execResult); err != nil {
		t.Fatal(err)
	}
	if execResult.Stdout != "ok\n" || execResult.ExitCode != 0 {
		t.Fatalf("unexpected exec response: %#v", execResult)
	}
}

func TestPortExposureReturnsRouteURL(t *testing.T) {
	t.Setenv("DAYTONA_SANDBOX_NAME", "agent")
	t.Setenv("DAYTONA_WORKLOAD_CONTAINER", "workload")
	t.Setenv("DAYTONA_ROUTE_BASE_URL", "https://sandbox-api.tailnet")
	server := New()
	server.now = func() time.Time { return time.Unix(10, 0) }

	req := httptest.NewRequest(http.MethodPost, "/ports", strings.NewReader(`{"name":"http","port":8080}`))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var exposure PortExposure
	if err := json.NewDecoder(res.Body).Decode(&exposure); err != nil {
		t.Fatal(err)
	}
	if exposure.URL != "https://sandbox-api.tailnet/sandboxes/agent/ports/http" {
		t.Fatalf("unexpected route URL %q", exposure.URL)
	}
}

func TestSSHDetails(t *testing.T) {
	t.Setenv("DAYTONA_SANDBOX_NAME", "agent")
	t.Setenv("DAYTONA_WORKLOAD_CONTAINER", "workload")
	t.Setenv("DAYTONA_SSH_ENABLED", "true")
	t.Setenv("DAYTONA_SSH_HOST", "agent.tailnet")
	t.Setenv("DAYTONA_SSH_PORT", "2222")
	t.Setenv("DAYTONA_SSH_USERNAME", "coder")
	server := New()

	req := httptest.NewRequest(http.MethodGet, "/ssh", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var ssh SSHResponse
	if err := json.NewDecoder(res.Body).Decode(&ssh); err != nil {
		t.Fatal(err)
	}
	if !ssh.Enabled || ssh.Command != "ssh -p 2222 coder@agent.tailnet" {
		t.Fatalf("unexpected ssh response: %#v", ssh)
	}
}

func TestFileWriteRejectsInvalidBase64(t *testing.T) {
	t.Setenv("DAYTONA_SANDBOX_NAME", "agent")
	t.Setenv("DAYTONA_WORKLOAD_CONTAINER", "workload")
	server := New()

	req := httptest.NewRequest(http.MethodPost, "/files", strings.NewReader(`{"path":"/tmp/file","contentBase64":"%%%INVALID%%%"}`))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest && res.Code != http.StatusBadGateway {
		t.Fatalf("expected file handler to reject invalid write, got %d: %s", res.Code, res.Body.String())
	}
}

type fakeExecutor struct{}

func (fakeExecutor) Exec(_ context.Context, req ExecRequest) (ExecResponse, error) {
	return ExecResponse{
		ExitCode:   0,
		Stdout:     "ok\n",
		StartedAt:  time.Unix(1, 0),
		FinishedAt: time.Unix(2, 0),
	}, nil
}
