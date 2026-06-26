package toolbox

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Identity struct {
	SandboxName       string    `json:"sandboxName"`
	WorkloadContainer string    `json:"workloadContainer"`
	SSHEnabled        bool      `json:"sshEnabled"`
	CredentialVersion string    `json:"credentialVersion,omitempty"`
	RouteBaseURL      string    `json:"routeBaseUrl,omitempty"`
	DopplerProject    string    `json:"dopplerProject,omitempty"`
	DopplerConfig     string    `json:"dopplerConfig,omitempty"`
	ReloadedAt        time.Time `json:"reloadedAt"`
}

type Server struct {
	now      func() time.Time
	executor Executor

	mu    sync.Mutex
	ports map[string]PortExposure
}

func New() *Server {
	return &Server{
		now:      time.Now,
		executor: WorkloadExecutor{},
		ports:    map[string]PortExposure{},
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := s.now()
	switch r.URL.Path {
	case "/healthz":
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case "/readyz":
		s.ready(w, r)
	case "/identity":
		s.identity(w, r)
	case "/exec":
		s.exec(w, r)
	case "/files":
		s.files(w, r)
	case "/ports":
		s.portControl(w, r)
	case "/ssh":
		s.ssh(w, r)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
	log.Printf("toolbox request sandbox=%q method=%s path=%s duration=%s", os.Getenv("DAYTONA_SANDBOX_NAME"), r.Method, r.URL.Path, s.now().Sub(start))
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity, err := s.loadIdentity()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if identity.WorkloadContainer == "" {
		writeError(w, http.StatusServiceUnavailable, "DAYTONA_WORKLOAD_CONTAINER is required")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity, err := s.loadIdentity()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, identity)
}

type ExecRequest struct {
	Command        []string          `json:"command"`
	Stdin          string            `json:"stdin,omitempty"`
	Env            map[string]string `json:"env,omitempty"`
	Workdir        string            `json:"workdir,omitempty"`
	Target         string            `json:"target,omitempty"`
	TimeoutSeconds int               `json:"timeoutSeconds,omitempty"`
}

type ExecResponse struct {
	ExitCode   int       `json:"exitCode"`
	Stdout     string    `json:"stdout,omitempty"`
	Stderr     string    `json:"stderr,omitempty"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
}

type Executor interface {
	Exec(ctx context.Context, req ExecRequest) (ExecResponse, error)
}

type WorkloadExecutor struct{}

func (WorkloadExecutor) Exec(ctx context.Context, req ExecRequest) (ExecResponse, error) {
	if len(req.Command) == 0 || req.Command[0] == "" {
		return ExecResponse{}, errors.New("command is required")
	}
	if req.Target == "" {
		req.Target = "workload"
	}

	command := append([]string(nil), req.Command...)
	if req.Target == "workload" {
		pid, err := workloadPID(os.Getenv("DAYTONA_WORKLOAD_CONTAINER"))
		if err != nil {
			return ExecResponse{}, err
		}
		command = append([]string{"nsenter", "--target", pid, "--mount", "--uts", "--ipc", "--net", "--pid", "--"}, command...)
	}

	started := time.Now().UTC()
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	if req.Workdir != "" {
		cmd.Dir = req.Workdir
	}
	cmd.Env = os.Environ()
	for key, value := range req.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	if req.Stdin != "" {
		cmd.Stdin = strings.NewReader(req.Stdin)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	finished := time.Now().UTC()

	response := ExecResponse{
		ExitCode:   0,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		StartedAt:  started,
		FinishedAt: finished,
	}
	if err == nil {
		return response, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		response.ExitCode = exitErr.ExitCode()
		return response, nil
	}
	return response, err
}

func (s *Server) exec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if _, err := s.loadIdentity(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	var req ExecRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	timeout := time.Duration(req.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	if timeout > 10*time.Minute {
		writeError(w, http.StatusBadRequest, "timeoutSeconds must be at most 600")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	res, err := s.executor.Exec(ctx, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type FileRequest struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"contentBase64,omitempty"`
	Mode          uint32 `json:"mode,omitempty"`
}

type FileResponse struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"contentBase64,omitempty"`
	Size          int64  `json:"size"`
}

func (s *Server) files(w http.ResponseWriter, r *http.Request) {
	if _, err := s.loadIdentity(); err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	switch r.Method {
	case http.MethodGet:
		path := r.URL.Query().Get("path")
		if path == "" {
			writeError(w, http.StatusBadRequest, "path is required")
			return
		}
		s.readFile(w, path)
	case http.MethodPost:
		var req FileRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.writeFile(w, req)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) readFile(w http.ResponseWriter, path string) {
	resolved, err := workloadPath(path)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, FileResponse{
		Path:          path,
		ContentBase64: base64.StdEncoding.EncodeToString(data),
		Size:          int64(len(data)),
	})
}

func (s *Server) writeFile(w http.ResponseWriter, req FileRequest) {
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	resolved, err := workloadPath(req.Path)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.ContentBase64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "contentBase64 is invalid")
		return
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	mode := os.FileMode(req.Mode)
	if mode == 0 {
		mode = 0o644
	}
	if err := os.WriteFile(resolved, data, mode); err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, FileResponse{Path: req.Path, Size: int64(len(data))})
}

type PortExposeRequest struct {
	Name     string `json:"name"`
	Port     int32  `json:"port"`
	Protocol string `json:"protocol,omitempty"`
}

type PortExposure struct {
	Name      string    `json:"name"`
	Port      int32     `json:"port"`
	Protocol  string    `json:"protocol"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

func (s *Server) portControl(w http.ResponseWriter, r *http.Request) {
	identity, err := s.loadIdentity()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	switch r.Method {
	case http.MethodGet:
		s.mu.Lock()
		exposures := make([]PortExposure, 0, len(s.ports))
		for _, exposure := range s.ports {
			exposures = append(exposures, exposure)
		}
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, exposures)
	case http.MethodPost:
		var req PortExposeRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if req.Port <= 0 {
			writeError(w, http.StatusBadRequest, "port must be positive")
			return
		}
		protocol := req.Protocol
		if protocol == "" {
			protocol = "TCP"
		}
		exposure := PortExposure{
			Name:      req.Name,
			Port:      req.Port,
			Protocol:  protocol,
			URL:       routeURL(identity.RouteBaseURL, identity.SandboxName, req.Name),
			CreatedAt: s.now().UTC(),
		}
		s.mu.Lock()
		s.ports[req.Name] = exposure
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, exposure)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

type SSHResponse struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Command  string `json:"command,omitempty"`
}

func (s *Server) ssh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	identity, err := s.loadIdentity()
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	if !identity.SSHEnabled {
		writeJSON(w, http.StatusOK, SSHResponse{Enabled: false})
		return
	}
	host := os.Getenv("DAYTONA_SSH_HOST")
	if host == "" {
		host = identity.SandboxName + ".sandbox.tailnet"
	}
	port := envInt("DAYTONA_SSH_PORT", 22)
	username := os.Getenv("DAYTONA_SSH_USERNAME")
	if username == "" {
		username = "daytona"
	}
	writeJSON(w, http.StatusOK, SSHResponse{
		Enabled:  true,
		Host:     host,
		Port:     port,
		Username: username,
		Command:  fmt.Sprintf("ssh -p %d %s@%s", port, username, host),
	})
}

func (s *Server) loadIdentity() (Identity, error) {
	sandboxName := os.Getenv("DAYTONA_SANDBOX_NAME")
	if sandboxName == "" {
		return Identity{}, errMissingSandboxName{}
	}
	sshEnabled, _ := strconv.ParseBool(os.Getenv("DAYTONA_SSH_ENABLED"))
	return Identity{
		SandboxName:       sandboxName,
		WorkloadContainer: os.Getenv("DAYTONA_WORKLOAD_CONTAINER"),
		SSHEnabled:        sshEnabled,
		CredentialVersion: os.Getenv("DAYTONA_CREDENTIAL_VERSION"),
		RouteBaseURL:      os.Getenv("DAYTONA_ROUTE_BASE_URL"),
		DopplerProject:    os.Getenv("DAYTONA_DOPPLER_PROJECT"),
		DopplerConfig:     os.Getenv("DAYTONA_DOPPLER_CONFIG"),
		ReloadedAt:        s.now().UTC(),
	}, nil
}

type errMissingSandboxName struct{}

func (errMissingSandboxName) Error() string {
	return "DAYTONA_SANDBOX_NAME is required"
}

func workloadPID(containerName string) (string, error) {
	if containerName == "" {
		return "", errors.New("DAYTONA_WORKLOAD_CONTAINER is required")
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return "", err
	}
	needle := "DAYTONA_CONTAINER_NAME=" + containerName
	fallback := ""
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid := entry.Name()
		if _, err := strconv.Atoi(pid); err != nil {
			continue
		}
		data, err := os.ReadFile(filepath.Join("/proc", pid, "environ"))
		if err == nil && strings.Contains(string(data), needle) {
			return pid, nil
		}
		if fallback != "" {
			continue
		}
		if isWorkloadPIDCandidate(pid) {
			fallback = pid
		}
	}
	if fallback != "" {
		return fallback, nil
	}
	return "", fmt.Errorf("workload process for container %q not found", containerName)
}

func isWorkloadPIDCandidate(pid string) bool {
	comm, err := os.ReadFile(filepath.Join("/proc", pid, "comm"))
	if err != nil {
		return false
	}
	name := strings.TrimSpace(string(comm))
	switch name {
	case "", "pause", "toolbox-sidecar":
		return false
	default:
		return true
	}
}

func workloadPath(path string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(path, "/"))
	if clean == "/" {
		return "", errors.New("path must not be root")
	}
	pid, err := workloadPID(os.Getenv("DAYTONA_WORKLOAD_CONTAINER"))
	if err != nil {
		return "", err
	}
	return filepath.Join("/proc", pid, "root", clean), nil
}

func routeURL(baseURL string, sandboxName string, portName string) string {
	if baseURL == "" {
		baseURL = "http://" + net.JoinHostPort(sandboxName+".sandbox.tailnet", "80")
	}
	return strings.TrimRight(baseURL, "/") + "/sandboxes/" + sandboxName + "/ports/" + portName
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
