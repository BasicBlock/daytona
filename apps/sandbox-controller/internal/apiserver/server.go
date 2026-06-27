package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Server struct {
	client        client.Client
	namespace     string
	logStreamer   PodLogStreamer
	podExecutor   PodExecutor
	publicBaseURL string
}

type PodLogStreamer interface {
	StreamPodLogs(ctx context.Context, namespace string, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error)
}

type KubernetesPodLogStreamer struct {
	clientset kubernetes.Interface
}

func NewKubernetesPodLogStreamer(clientset kubernetes.Interface) *KubernetesPodLogStreamer {
	return &KubernetesPodLogStreamer{clientset: clientset}
}

func (s *KubernetesPodLogStreamer) StreamPodLogs(ctx context.Context, namespace string, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	return s.clientset.CoreV1().Pods(namespace).GetLogs(podName, options).Stream(ctx)
}

type Option func(*Server)

func WithPodLogStreamer(streamer PodLogStreamer) Option {
	return func(s *Server) {
		s.logStreamer = streamer
	}
}

func WithPodExecutor(executor PodExecutor) Option {
	return func(s *Server) {
		if executor != nil {
			s.podExecutor = executor
		}
	}
}

func WithPublicBaseURL(baseURL string) Option {
	return func(s *Server) {
		s.publicBaseURL = strings.TrimRight(baseURL, "/")
	}
}

func New(client client.Client, namespace string, options ...Option) *Server {
	if namespace == "" {
		namespace = "sandboxes"
	}
	server := &Server{
		client:        client,
		namespace:     namespace,
		publicBaseURL: strings.TrimRight(envDefault("DAYTONA_PUBLIC_BASE_URL", "http://sandbox-api.daytona-system.svc.cluster.local:8090"), "/"),
	}
	for _, option := range options {
		option(server)
	}
	return server
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if r.URL.Path == "/sandboxes" {
		switch r.Method {
		case http.MethodGet:
			s.listSandboxes(w, r)
		case http.MethodPost:
			s.createSandbox(w, r)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if strings.HasPrefix(r.URL.Path, "/sandboxes/") {
		s.handleSandbox(w, r)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

type SandboxRequest struct {
	Name string                `json:"name"`
	Spec computev1.SandboxSpec `json:"spec"`
}

type StopRequest struct {
	SnapshotBeforeStop bool                             `json:"snapshotBeforeStop,omitempty"`
	SnapshotName       string                           `json:"snapshotName,omitempty"`
	Provider           computev1.SnapshotProvider       `json:"provider,omitempty"`
	GKE                computev1.GKEPodSnapshotSpec     `json:"gke,omitempty"`
	Local              computev1.LocalRunscProviderSpec `json:"local,omitempty"`
}

type SnapshotRequest struct {
	Name     string                           `json:"name"`
	Provider computev1.SnapshotProvider       `json:"provider,omitempty"`
	GKE      computev1.GKEPodSnapshotSpec     `json:"gke,omitempty"`
	Local    computev1.LocalRunscProviderSpec `json:"local,omitempty"`
}

type ForkRequest struct {
	Name         string `json:"name"`
	SnapshotName string `json:"snapshotName,omitempty"`
}

type AccessResponse struct {
	SandboxName string       `json:"sandboxName"`
	Phase       string       `json:"phase"`
	ServiceName string       `json:"serviceName"`
	ToolboxURL  string       `json:"toolboxUrl"`
	Ports       []PortAccess `json:"ports"`
}

type PortAccess struct {
	Name string `json:"name"`
	Port int32  `json:"port"`
	URL  string `json:"url"`
}

type PortExposeRequest struct {
	Name             string `json:"name"`
	Port             int32  `json:"port"`
	Protocol         string `json:"protocol,omitempty"`
	Signed           bool   `json:"signed,omitempty"`
	ExpiresInSeconds int    `json:"expiresInSeconds,omitempty"`
}

type PortExposure struct {
	Name      string    `json:"name"`
	Port      int32     `json:"port"`
	Protocol  string    `json:"protocol"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

type SSHResponse struct {
	Enabled  bool   `json:"enabled"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Username string `json:"username,omitempty"`
	Command  string `json:"command,omitempty"`
}

func (s *Server) listSandboxes(w http.ResponseWriter, r *http.Request) {
	var list computev1.SandboxList
	if err := s.client.List(r.Context(), &list, client.InNamespace(s.namespace)); err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, list.Items)
}

func (s *Server) createSandbox(w http.ResponseWriter, r *http.Request) {
	var req SandboxRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateName("name", req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateSandboxSpec(req.Spec); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Spec.DesiredState == "" {
		req.Spec.DesiredState = computev1.SandboxDesiredStateRunning
	}

	sandbox := &computev1.Sandbox{
		TypeMeta: metav1.TypeMeta{
			APIVersion: computev1.SchemeGroupVersion.String(),
			Kind:       "Sandbox",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      req.Name,
			Namespace: s.namespace,
		},
		Spec: req.Spec,
	}
	if err := s.client.Create(r.Context(), sandbox); err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sandbox)
}

func (s *Server) handleSandbox(w http.ResponseWriter, r *http.Request) {
	remainder := strings.TrimPrefix(r.URL.Path, "/sandboxes/")
	switch {
	case strings.HasSuffix(remainder, ":start"):
		s.startSandbox(w, r, strings.TrimSuffix(remainder, ":start"))
	case strings.HasSuffix(remainder, ":stop"):
		s.stopSandbox(w, r, strings.TrimSuffix(remainder, ":stop"))
	case strings.HasSuffix(remainder, ":snapshot"):
		s.createSnapshot(w, r, strings.TrimSuffix(remainder, ":snapshot"))
	case strings.HasSuffix(remainder, ":fork"):
		s.forkSandbox(w, r, strings.TrimSuffix(remainder, ":fork"))
	case strings.HasSuffix(remainder, "/exec"):
		s.execSandbox(w, r, strings.TrimSuffix(remainder, "/exec"))
	case strings.HasSuffix(remainder, "/files"):
		writeError(w, http.StatusNotImplemented, "sandbox file API is not available without a toolbox sidecar")
	case strings.HasSuffix(remainder, "/ports"):
		s.portControl(w, r, strings.TrimSuffix(remainder, "/ports"))
	case strings.HasSuffix(remainder, "/ssh"):
		s.sshSandbox(w, r, strings.TrimSuffix(remainder, "/ssh"))
	case strings.HasSuffix(remainder, "/logs"):
		s.getLogs(w, r, strings.TrimSuffix(remainder, "/logs"))
	case strings.HasSuffix(remainder, "/access"):
		s.getAccess(w, r, strings.TrimSuffix(remainder, "/access"))
	default:
		switch r.Method {
		case http.MethodGet:
			s.getSandbox(w, r, remainder)
		case http.MethodDelete:
			s.deleteSandbox(w, r, remainder)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}
}

func (s *Server) getSandbox(w http.ResponseWriter, r *http.Request, name string) {
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sandbox)
}

func (s *Server) deleteSandbox(w http.ResponseWriter, r *http.Request, name string) {
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	if err := s.client.Delete(r.Context(), sandbox); err != nil {
		writeKubernetesError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) stopSandbox(w http.ResponseWriter, r *http.Request, name string) {
	var req StopRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	if err := validateProvider(req.Provider); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	mutate := func(sandbox *computev1.Sandbox) {
		sandbox.Spec.DesiredState = computev1.SandboxDesiredStateStopped
		if req.SnapshotBeforeStop || req.SnapshotName != "" {
			sandbox.Spec.StopPolicy = computev1.SandboxStopPolicySpec{
				SnapshotBeforeStop: true,
				SnapshotName:       req.SnapshotName,
				AutoStopMinutes:    sandbox.Spec.StopPolicy.AutoStopMinutes,
				Provider:           req.Provider,
				GKE:                req.GKE,
				Local:              req.Local,
			}
		} else {
			sandbox.Spec.StopPolicy.SnapshotBeforeStop = false
			sandbox.Spec.StopPolicy.SnapshotName = ""
			sandbox.Spec.StopPolicy.Provider = ""
			sandbox.Spec.StopPolicy.GKE = computev1.GKEPodSnapshotSpec{}
			sandbox.Spec.StopPolicy.Local = computev1.LocalRunscProviderSpec{}
		}
	}
	s.patchDesiredState(w, r, name, computev1.SandboxDesiredStateStopped, mutate)
}

func (s *Server) startSandbox(w http.ResponseWriter, r *http.Request, name string) {
	s.patchDesiredState(w, r, name, computev1.SandboxDesiredStateRunning, func(sandbox *computev1.Sandbox) {
		prepareWake(sandbox)
	})
}

func (s *Server) patchDesiredState(w http.ResponseWriter, r *http.Request, name string, desired computev1.SandboxDesiredState, mutate func(*computev1.Sandbox)) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	if desired == computev1.SandboxDesiredStateRunning {
		if err := s.touchSandboxActivity(r.Context(), sandbox.Name); err != nil {
			writeKubernetesError(w, err)
			return
		}
		sandbox, err = s.getSandboxObject(r.Context(), name)
		if err != nil {
			writeKubernetesError(w, err)
			return
		}
	}
	sandbox.Spec.DesiredState = desired
	if mutate != nil {
		mutate(sandbox)
	}
	if err := s.client.Update(r.Context(), sandbox); err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sandbox)
}

func (s *Server) createSnapshot(w http.ResponseWriter, r *http.Request, sandboxName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req SnapshotRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateName("name", req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateProvider(req.Provider); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := s.getSandboxObject(r.Context(), sandboxName); err != nil {
		writeKubernetesError(w, err)
		return
	}
	provider := req.Provider
	if provider == "" {
		provider = computev1.SnapshotProviderGKEPodSnapshot
	}
	snapshot := &computev1.SandboxSnapshot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: computev1.SchemeGroupVersion.String(),
			Kind:       "SandboxSnapshot",
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: s.namespace},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: provider,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: sandboxName},
			GKE:      req.GKE,
			Local:    req.Local,
		},
	}
	if err := s.client.Create(r.Context(), snapshot); err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, snapshot)
}

func (s *Server) forkSandbox(w http.ResponseWriter, r *http.Request, sourceSandboxName string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req ForkRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateName("name", req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	snapshot, err := s.resolveForkSnapshot(r.Context(), sourceSandboxName, req.SnapshotName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	source, err := s.getSandboxObject(r.Context(), snapshot.Spec.Source.SandboxName)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}

	spec := source.Spec.DeepCopy()
	spec.DesiredState = computev1.SandboxDesiredStateRunning
	spec.StopPolicy = computev1.SandboxStopPolicySpec{}
	spec.Restore = &computev1.SandboxSnapshotRestoreRef{
		Name:               snapshot.Name,
		Provider:           snapshot.Spec.Provider,
		ProviderObjectName: snapshot.Status.ProviderObjectName,
		StorageRef:         snapshot.Status.StorageRef,
	}

	fork := &computev1.Sandbox{
		TypeMeta: metav1.TypeMeta{
			APIVersion: computev1.SchemeGroupVersion.String(),
			Kind:       "Sandbox",
		},
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: s.namespace},
		Spec:       spec,
	}
	if err := s.client.Create(r.Context(), fork); err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, fork)
}

func (s *Server) getAccess(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	serviceName := render.ServiceName(sandbox)
	if sandbox.Status.ServiceName != "" {
		serviceName = sandbox.Status.ServiceName
	}
	ports := make([]PortAccess, 0, len(sandbox.Spec.Ports))
	for _, port := range sandbox.Spec.Ports {
		ports = append(ports, PortAccess{
			Name: port.Name,
			Port: port.Port,
			URL:  fmt.Sprintf("%s/sandboxes/%s/ports/%s", s.publicBaseURL, sandbox.Name, port.Name),
		})
	}
	writeJSON(w, http.StatusOK, AccessResponse{
		SandboxName: sandbox.Name,
		Phase:       string(sandbox.Status.Phase),
		ServiceName: serviceName,
		ToolboxURL:  "",
		Ports:       ports,
	})
}

func (s *Server) getLogs(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.logStreamer == nil {
		writeError(w, http.StatusServiceUnavailable, "pod log streaming is not configured")
		return
	}
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	options, err := parsePodLogOptions(r.URL.Query())
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	podName := sandbox.Status.PodName
	if podName == "" {
		podName = render.PodName(sandbox)
	}
	namespace := sandbox.Namespace
	if namespace == "" {
		namespace = s.namespace
	}

	logs, err := s.logStreamer.StreamPodLogs(r.Context(), namespace, podName, options)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	defer logs.Close()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_ = copyLogs(w, logs, options.Follow)
}

func (s *Server) execSandbox(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}

	if render.DesiredState(sandbox) == computev1.SandboxDesiredStateStopped {
		if _, err := s.wakeSandbox(r.Context(), sandbox); err != nil {
			writeKubernetesError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "starting",
			"message": "sandbox was stopped and is starting before the request can be retried",
		})
		return
	}

	if s.podExecutor == nil {
		writeError(w, http.StatusServiceUnavailable, "pod executor is not configured")
		return
	}

	if err := s.touchSandboxActivity(r.Context(), sandbox.Name); err != nil {
		writeKubernetesError(w, err)
		return
	}
	podName := sandbox.Status.PodName
	if podName == "" {
		podName = render.PodName(sandbox)
	}
	namespace := sandbox.Namespace
	if namespace == "" {
		namespace = s.namespace
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()
	res, err := s.podExecutor.Exec(ctx, namespace, podName, render.WorkloadContainerName, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) sshSandbox(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	if render.DesiredState(sandbox) == computev1.SandboxDesiredStateStopped {
		if _, err := s.wakeSandbox(r.Context(), sandbox); err != nil {
			writeKubernetesError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]string{
			"status":  "starting",
			"message": "sandbox was stopped and is starting before the request can be retried",
		})
		return
	}
	if err := s.touchSandboxActivity(r.Context(), sandbox.Name); err != nil {
		writeKubernetesError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sandboxSSHResponse(sandbox))
}

func (s *Server) portControl(w http.ResponseWriter, r *http.Request, name string) {
	sandbox, err := s.getSandboxObject(r.Context(), name)
	if err != nil {
		writeKubernetesError(w, err)
		return
	}
	switch r.Method {
	case http.MethodGet:
		exposures := make([]PortExposure, 0, len(sandbox.Spec.Ports))
		for _, port := range sandbox.Spec.Ports {
			exposures = append(exposures, PortExposure{
				Name:      port.Name,
				Port:      port.Port,
				Protocol:  string(portProtocol(port.Protocol)),
				URL:       portURL(s.publicBaseURL, sandbox.Name, port.Name),
				CreatedAt: time.Now().UTC(),
			})
		}
		writeJSON(w, http.StatusOK, exposures)
	case http.MethodPost:
		var req PortExposeRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Name == "" {
			req.Name = fmt.Sprintf("p%d", req.Port)
		}
		if errs := kvalidation.IsDNS1123Label(req.Name); len(errs) > 0 {
			writeError(w, http.StatusBadRequest, fmt.Sprintf("name must be a DNS-1123 label: %s", strings.Join(errs, "; ")))
			return
		}
		if req.Port <= 0 || req.Port > 65535 {
			writeError(w, http.StatusBadRequest, "port must be between 1 and 65535")
			return
		}
		protocol := req.Protocol
		if protocol == "" {
			protocol = string(corev1.ProtocolTCP)
		}
		writeJSON(w, http.StatusOK, PortExposure{
			Name:      req.Name,
			Port:      req.Port,
			Protocol:  protocol,
			URL:       portURL(s.publicBaseURL, sandbox.Name, req.Name),
			CreatedAt: time.Now().UTC(),
		})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func prepareWake(sandbox *computev1.Sandbox) {
	if sandbox.Status.SleepSnapshotName != "" {
		sandbox.Spec.Restore = &computev1.SandboxSnapshotRestoreRef{Name: sandbox.Status.SleepSnapshotName}
	}
	if sandbox.Spec.StopPolicy.SnapshotBeforeStop {
		sandbox.Spec.StopPolicy.SnapshotName = ""
	}
}

func (s *Server) touchSandboxActivity(ctx context.Context, name string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		current, err := s.getSandboxObject(ctx, name)
		if err != nil {
			return err
		}
		now := metav1.Now()
		next := current.DeepCopyObject().(*computev1.Sandbox)
		next.Status.LastActivityTime = &now
		return s.client.Status().Patch(ctx, next, client.MergeFrom(current))
	})
}

func (s *Server) wakeSandbox(ctx context.Context, sandbox *computev1.Sandbox) (*computev1.Sandbox, error) {
	if err := s.touchSandboxActivity(ctx, sandbox.Name); err != nil {
		return nil, err
	}
	current, err := s.getSandboxObject(ctx, sandbox.Name)
	if err != nil {
		return nil, err
	}
	current.Spec.DesiredState = computev1.SandboxDesiredStateRunning
	prepareWake(current)
	if err := s.client.Update(ctx, current); err != nil {
		return nil, err
	}
	return current, nil
}

func sandboxSSHResponse(sandbox *computev1.Sandbox) SSHResponse {
	if !sandbox.Spec.Access.SSHEnabled {
		return SSHResponse{Enabled: false}
	}
	host := sandbox.Name + ".sandbox.tailnet"
	port := 22
	username := "daytona"
	return SSHResponse{
		Enabled:  true,
		Host:     host,
		Port:     port,
		Username: username,
		Command:  fmt.Sprintf("ssh -p %d %s@%s", port, username, host),
	}
}

func portProtocol(protocol corev1.Protocol) corev1.Protocol {
	if protocol == "" {
		return corev1.ProtocolTCP
	}
	return protocol
}

func portURL(baseURL string, sandboxName string, portName string) string {
	return strings.TrimRight(baseURL, "/") + "/sandboxes/" + sandboxName + "/ports/" + portName
}

func (s *Server) getSandboxObject(ctx context.Context, name string) (*computev1.Sandbox, error) {
	var sandbox computev1.Sandbox
	if err := s.client.Get(ctx, types.NamespacedName{Name: name, Namespace: s.namespace}, &sandbox); err != nil {
		return nil, err
	}
	return &sandbox, nil
}

func (s *Server) resolveForkSnapshot(ctx context.Context, sourceSandboxName string, snapshotName string) (*computev1.SandboxSnapshot, error) {
	if snapshotName != "" {
		var snapshot computev1.SandboxSnapshot
		if err := s.client.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: s.namespace}, &snapshot); err != nil {
			return nil, err
		}
		return readySnapshotOrError(&snapshot)
	}

	var list computev1.SandboxSnapshotList
	if err := s.client.List(ctx, &list, client.InNamespace(s.namespace)); err != nil {
		return nil, err
	}
	candidates := make([]computev1.SandboxSnapshot, 0)
	for _, item := range list.Items {
		if item.Spec.Source.SandboxName == sourceSandboxName && item.Status.Phase == computev1.SandboxSnapshotPhaseReady && item.Status.ProviderObjectName != "" {
			candidates = append(candidates, item)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].CreationTimestamp.After(candidates[j].CreationTimestamp.Time)
	})
	if len(candidates) == 0 {
		return nil, errors.New("no ready snapshot found for source sandbox")
	}
	return &candidates[0], nil
}

func readySnapshotOrError(snapshot *computev1.SandboxSnapshot) (*computev1.SandboxSnapshot, error) {
	if snapshot.Status.Phase != computev1.SandboxSnapshotPhaseReady || snapshot.Status.ProviderObjectName == "" {
		return nil, fmt.Errorf("snapshot %s is not ready", snapshot.Name)
	}
	return snapshot, nil
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeKubernetesError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if apierrors.IsNotFound(err) {
		status = http.StatusNotFound
	} else if apierrors.IsAlreadyExists(err) || apierrors.IsConflict(err) {
		status = http.StatusConflict
	} else if apierrors.IsInvalid(err) || apierrors.IsBadRequest(err) {
		status = http.StatusBadRequest
	}
	writeError(w, status, err.Error())
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func validateName(field string, name string) error {
	if name == "" {
		return fmt.Errorf("%s is required", field)
	}
	if errs := kvalidation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("%s must be a DNS-1123 label: %s", field, strings.Join(errs, "; "))
	}
	return nil
}

func validateSandboxSpec(spec computev1.SandboxSpec) error {
	if spec.Image == "" {
		return errors.New("spec.image is required")
	}
	for _, port := range spec.Ports {
		if port.Name == "" {
			return errors.New("spec.ports[].name is required")
		}
		if errs := kvalidation.IsDNS1123Label(port.Name); len(errs) > 0 {
			return fmt.Errorf("spec.ports[%s].name must be a DNS-1123 label: %s", port.Name, strings.Join(errs, "; "))
		}
		if port.Port <= 0 || port.Port > 65535 {
			return fmt.Errorf("spec.ports[%s].port must be between 1 and 65535", port.Name)
		}
	}
	if spec.Secrets.Provider != "" && spec.Secrets.Provider != "doppler" {
		return fmt.Errorf("spec.secrets.provider %q is unsupported", spec.Secrets.Provider)
	}
	return nil
}

func validateProvider(provider computev1.SnapshotProvider) error {
	if provider == "" || provider == computev1.SnapshotProviderGKEPodSnapshot || provider == computev1.SnapshotProviderLocalRunsc {
		return nil
	}
	return fmt.Errorf("provider %q is unsupported", provider)
}

func parsePodLogOptions(values url.Values) (*corev1.PodLogOptions, error) {
	options := &corev1.PodLogOptions{Container: render.WorkloadContainerName}
	if container := values.Get("container"); container != "" {
		if errs := kvalidation.IsDNS1123Label(container); len(errs) > 0 {
			return nil, fmt.Errorf("container must be a DNS-1123 label: %s", strings.Join(errs, "; "))
		}
		options.Container = container
	}

	follow, err := parseBoolQuery(values, "follow")
	if err != nil {
		return nil, err
	}
	options.Follow = follow

	previous, err := parseBoolQuery(values, "previous")
	if err != nil {
		return nil, err
	}
	options.Previous = previous

	timestamps, err := parseBoolQuery(values, "timestamps")
	if err != nil {
		return nil, err
	}
	options.Timestamps = timestamps

	tailLines, err := parseInt64Query(values, "tailLines", 0)
	if err != nil {
		return nil, err
	}
	options.TailLines = tailLines

	limitBytes, err := parseInt64Query(values, "limitBytes", 1)
	if err != nil {
		return nil, err
	}
	options.LimitBytes = limitBytes

	sinceSeconds, err := parseInt64Query(values, "sinceSeconds", 1)
	if err != nil {
		return nil, err
	}
	options.SinceSeconds = sinceSeconds

	if sinceTime := values.Get("sinceTime"); sinceTime != "" {
		parsed, err := time.Parse(time.RFC3339, sinceTime)
		if err != nil {
			return nil, errors.New("sinceTime must be an RFC3339 timestamp")
		}
		value := metav1.NewTime(parsed)
		options.SinceTime = &value
	}
	if options.SinceSeconds != nil && options.SinceTime != nil {
		return nil, errors.New("sinceSeconds and sinceTime cannot both be set")
	}

	return options, nil
}

func parseBoolQuery(values url.Values, name string) (bool, error) {
	raw := values.Get(name)
	if raw == "" {
		return false, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", name)
	}
	return value, nil
}

func parseInt64Query(values url.Values, name string, minimum int64) (*int64, error) {
	raw := values.Get(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value < minimum {
		return nil, fmt.Errorf("%s must be an integer >= %d", name, minimum)
	}
	return &value, nil
}

func copyLogs(w http.ResponseWriter, logs io.Reader, follow bool) error {
	flusher, canFlush := w.(http.Flusher)
	if !follow || !canFlush {
		_, err := io.Copy(w, logs)
		return err
	}

	buffer := make([]byte, 32*1024)
	for {
		n, readErr := logs.Read(buffer)
		if n > 0 {
			if _, err := w.Write(buffer[:n]); err != nil {
				return err
			}
			flusher.Flush()
		}
		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				return nil
			}
			return readErr
		}
	}
}

func envDefault(name string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	return value
}
