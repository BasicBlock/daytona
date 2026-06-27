package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestCreateAndStopSandbox(t *testing.T) {
	k8sClient := testClient(t)
	server := New(k8sClient, "sandboxes")

	createBody := `{"name":"agent","spec":{"image":"ubuntu:24.04"}}`
	res := request(t, server, http.MethodPost, "/sandboxes", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}

	stopBody := `{"force":false,"snapshotBeforeStop":true,"snapshotName":"agent-stop","gke":{"storageConfigName":"storage","postCheckpoint":"stop"}}`
	res = request(t, server, http.MethodPost, "/sandboxes/agent:stop", stopBody)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var sandbox computev1.Sandbox
	decode(t, res, &sandbox)
	if sandbox.Spec.DesiredState != computev1.SandboxDesiredStateStopped {
		t.Fatalf("expected stopped desired state, got %s", sandbox.Spec.DesiredState)
	}
	if !sandbox.Spec.StopPolicy.SnapshotBeforeStop || sandbox.Spec.StopPolicy.SnapshotName != "agent-stop" {
		t.Fatalf("expected stop snapshot policy, got %#v", sandbox.Spec.StopPolicy)
	}
}

func TestCreateSandboxEntryCommand(t *testing.T) {
	k8sClient := testClient(t)
	server := New(k8sClient, "sandboxes")

	createBody := `{"name":"agent","entryCommand":"nginx -g 'daemon off;'","spec":{"image":"nginx:1.27-alpine"}}`
	res := request(t, server, http.MethodPost, "/sandboxes", createBody)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}

	var response SandboxResponse
	decode(t, res, &response)
	if response.EntryCommand != "nginx -g 'daemon off;'" {
		t.Fatalf("expected entry command in response, got %q", response.EntryCommand)
	}
	if len(response.Spec.Command) != 2 || response.Spec.Command[0] != "/bin/sh" || response.Spec.Command[1] != "-lc" {
		t.Fatalf("expected shell command, got %#v", response.Spec.Command)
	}
	if len(response.Spec.Args) != 1 || response.Spec.Args[0] != "nginx -g 'daemon off;'" {
		t.Fatalf("expected entry command arg, got %#v", response.Spec.Args)
	}
}

func TestCreateSandboxRejectsAmbiguousEntryCommand(t *testing.T) {
	k8sClient := testClient(t)
	server := New(k8sClient, "sandboxes")

	createBody := `{"name":"agent","entryCommand":"sleep infinity","spec":{"image":"ubuntu:24.04","command":["/bin/sleep"],"args":["infinity"]}}`
	res := request(t, server, http.MethodPost, "/sandboxes", createBody)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "entryCommand cannot be used") {
		t.Fatalf("expected entry command conflict error, got %s", res.Body.String())
	}
}

func TestStopSandboxForceSkipsSnapshot(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			StopPolicy: computev1.SandboxStopPolicySpec{
				AutoStopMinutes: 60,
			},
		},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	stopBody := `{"force":true,"snapshotBeforeStop":true,"snapshotName":"agent-stop","gke":{"storageConfigName":"storage","postCheckpoint":"stop"}}`
	res := request(t, server, http.MethodPost, "/sandboxes/agent:stop", stopBody)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var sandbox computev1.Sandbox
	decode(t, res, &sandbox)
	if sandbox.Spec.DesiredState != computev1.SandboxDesiredStateStopped {
		t.Fatalf("expected stopped desired state, got %s", sandbox.Spec.DesiredState)
	}
	if sandbox.Spec.StopPolicy.SnapshotBeforeStop || sandbox.Spec.StopPolicy.SnapshotName != "" || sandbox.Spec.StopPolicy.Provider != "" {
		t.Fatalf("expected force stop to skip snapshot policy, got %#v", sandbox.Spec.StopPolicy)
	}
	if sandbox.Spec.StopPolicy.AutoStopMinutes != 60 {
		t.Fatalf("expected force stop to preserve auto-stop minutes, got %#v", sandbox.Spec.StopPolicy)
	}
}

func TestCreateSnapshot(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	body := `{"name":"agent-warm","gke":{"storageConfigName":"storage"}}`
	res := request(t, server, http.MethodPost, "/sandboxes/agent:snapshot", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}

	var snapshot computev1.SandboxSnapshot
	decode(t, res, &snapshot)
	if snapshot.Spec.Source.SandboxName != "agent" {
		t.Fatalf("expected source sandbox agent, got %q", snapshot.Spec.Source.SandboxName)
	}
}

func TestCreateLocalRunscSnapshot(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	body := `{"name":"agent-local","provider":"LocalRunsc","local":{"storage":{"mode":"filesystem","path":"/snapshots"}}}`
	res := request(t, server, http.MethodPost, "/sandboxes/agent:snapshot", body)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}

	var snapshot computev1.SandboxSnapshot
	decode(t, res, &snapshot)
	if snapshot.Spec.Provider != computev1.SnapshotProviderLocalRunsc {
		t.Fatalf("expected LocalRunsc provider, got %s", snapshot.Spec.Provider)
	}
	if snapshot.Spec.Local.Storage.Path != "/snapshots" {
		t.Fatalf("expected local storage path, got %#v", snapshot.Spec.Local.Storage)
	}
}

func TestForkSandboxUsesReadySnapshot(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-warm", Namespace: "sandboxes"},
		Spec: computev1.SandboxSnapshotSpec{
			Source: computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
		},
		Status: computev1.SandboxSnapshotStatus{
			Phase:              computev1.SandboxSnapshotPhaseReady,
			ProviderObjectName: "gke-snapshot",
		},
	}
	k8sClient := testClient(t, source, snapshot)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodPost, "/sandboxes/agent:fork", `{"name":"fork","snapshotName":"agent-warm"}`)
	if res.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", res.Code, res.Body.String())
	}

	var fork computev1.Sandbox
	decode(t, res, &fork)
	if fork.Spec.Restore == nil || fork.Spec.Restore.ProviderObjectName != "gke-snapshot" {
		t.Fatalf("expected restore ref from ready snapshot, got %#v", fork.Spec.Restore)
	}
}

func TestExecUsesPodExecutor(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status: computev1.SandboxStatus{
			Phase:       computev1.SandboxPhaseRunning,
			PodName:     "sandbox-agent",
			ServiceName: "sandbox-agent",
		},
	}
	k8sClient := testClient(t, source)
	executor := &fakePodExecutor{response: ExecResponse{ExitCode: 0, Stdout: "ok\n"}}
	server := New(k8sClient, "sandboxes", WithPodExecutor(executor))
	res := request(t, server, http.MethodPost, "/sandboxes/agent/exec", `{"command":["echo","ok"]}`)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if executor.namespace != "sandboxes" || executor.podName != "sandbox-agent" || executor.containerName != render.WorkloadContainerName {
		t.Fatalf("unexpected executor target namespace=%s pod=%s container=%s", executor.namespace, executor.podName, executor.containerName)
	}
	if len(executor.request.Command) != 2 || executor.request.Command[0] != "echo" || executor.request.Command[1] != "ok" {
		t.Fatalf("unexpected exec request %#v", executor.request)
	}
}

func TestAccessReturnsPublicPortRoutes(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Ports: []computev1.SandboxPort{{
				Name: "http",
				Port: 8080,
			}},
		},
		Status: computev1.SandboxStatus{ServiceName: "sandbox-agent"},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes", WithPublicBaseURL("https://sandbox-api.tailnet"))

	res := request(t, server, http.MethodGet, "/sandboxes/agent/access", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var access AccessResponse
	decode(t, res, &access)
	if len(access.Ports) != 1 || access.Ports[0].URL != "https://sandbox-api.tailnet/sandboxes/agent/ports/http" {
		t.Fatalf("unexpected access response: %#v", access)
	}
}

func TestAccessOmitsServiceNameWithoutPorts(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status:     computev1.SandboxStatus{Phase: computev1.SandboxPhaseRunning},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes", WithPublicBaseURL("https://sandbox-api.tailnet"))

	res := request(t, server, http.MethodGet, "/sandboxes/agent/access", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var access AccessResponse
	decode(t, res, &access)
	if access.ServiceName != "" {
		t.Fatalf("expected empty service name, got %q", access.ServiceName)
	}
	if len(access.Ports) != 0 {
		t.Fatalf("expected no ports, got %#v", access.Ports)
	}
}

func TestListSandboxesReturnsOpenPorts(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image:   "ubuntu:24.04",
			Command: []string{"/bin/sh", "-lc"},
			Args:    []string{"sleep infinity"},
			Ports: []computev1.SandboxPort{
				{Name: "http", Port: 8080},
				{Name: "metrics", Port: 9090, Protocol: corev1.ProtocolTCP},
			},
		},
		Status: computev1.SandboxStatus{Phase: computev1.SandboxPhaseRunning},
	}
	withoutPorts := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	k8sClient := testClient(t, source, withoutPorts)
	server := New(k8sClient, "sandboxes", WithPublicBaseURL("https://sandbox-api.tailnet"))

	res := request(t, server, http.MethodGet, "/sandboxes", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var sandboxes []SandboxListItem
	decode(t, res, &sandboxes)
	byName := map[string]SandboxListItem{}
	for _, sandbox := range sandboxes {
		byName[sandbox.Name] = sandbox
	}
	agent := byName["agent"]
	if len(agent.Ports) != 2 {
		t.Fatalf("expected two open ports, got %#v", agent.Ports)
	}
	if agent.Ports[0].Name != "http" || agent.Ports[0].Port != 8080 || agent.Ports[0].Protocol != "TCP" ||
		agent.Ports[0].URL != "https://sandbox-api.tailnet/sandboxes/agent/ports/http" {
		t.Fatalf("unexpected first port %#v", agent.Ports[0])
	}
	if ports := byName["worker"].Ports; len(ports) != 0 {
		t.Fatalf("expected worker to have no open ports, got %#v", ports)
	}
	if agent.EntryCommand != "sleep infinity" {
		t.Fatalf("expected list entry command, got %q", agent.EntryCommand)
	}
}

func TestPortControlPersistsExposure(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes", WithPublicBaseURL("https://sandbox-api.tailnet"))

	res := request(t, server, http.MethodPost, "/sandboxes/agent/ports", `{"name":"p80","port":80}`)
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "sandboxes", Name: "agent"}, &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Spec.Ports) != 1 || updated.Spec.Ports[0].Name != "p80" || updated.Spec.Ports[0].Port != 80 {
		t.Fatalf("expected persisted p80 exposure, got %#v", updated.Spec.Ports)
	}
}

func TestPortControlDeletesExposure(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Ports: []computev1.SandboxPort{
				{Name: "http", Port: 3000},
				{Name: "p80", Port: 80},
			},
		},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodDelete, "/sandboxes/agent/ports/p80", "")
	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", res.Code, res.Body.String())
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "sandboxes", Name: "agent"}, &updated); err != nil {
		t.Fatal(err)
	}
	if len(updated.Spec.Ports) != 1 || updated.Spec.Ports[0].Name != "http" || updated.Spec.Ports[0].Port != 3000 {
		t.Fatalf("expected only http exposure to remain, got %#v", updated.Spec.Ports)
	}
}

func TestProxySandboxPortRoutesToWorkloadPod(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/hello" || r.URL.RawQuery != "x=1" {
			t.Fatalf("unexpected upstream URL %s", r.URL.String())
		}
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portValue, err := net.SplitHostPort(upstreamURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseInt(portValue, 10, 32)
	if err != nil {
		t.Fatal(err)
	}

	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Ports: []computev1.SandboxPort{{
				Name: "web",
				Port: int32(port),
			}},
		},
		Status: computev1.SandboxStatus{
			Phase:   computev1.SandboxPhaseRunning,
			PodName: "sandbox-agent",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-agent", Namespace: "sandboxes"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: host,
		},
	}
	k8sClient := testClient(t, source, pod)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodGet, "/sandboxes/agent/ports/web/hello?x=1", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Header().Get("X-Upstream"); got != "ok" {
		t.Fatalf("expected upstream header, got %q", got)
	}
	if got := res.Body.String(); got != "proxied" {
		t.Fatalf("expected proxied body, got %q", got)
	}
}

func TestProxySandboxPortRequiresPersistedExposure(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status:     computev1.SandboxStatus{Phase: computev1.SandboxPhaseRunning},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodGet, "/sandboxes/agent/ports/p80", "")
	if res.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", res.Code, res.Body.String())
	}
}

func TestProxySandboxPortWakesStoppedSandbox(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			t.Fatalf("unexpected upstream URL %s", r.URL.String())
		}
		_, _ = w.Write([]byte("awake"))
	}))
	defer upstream.Close()

	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	host, portValue, err := net.SplitHostPort(upstreamURL.Host)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseInt(portValue, 10, 32)
	if err != nil {
		t.Fatal(err)
	}

	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
			Ports: []computev1.SandboxPort{{
				Name: "web",
				Port: int32(port),
			}},
		},
		Status: computev1.SandboxStatus{
			Phase:             computev1.SandboxPhaseRunning,
			PodName:           "sandbox-agent",
			SleepSnapshotName: "agent-sleep",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-agent", Namespace: "sandboxes"},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: host,
		},
	}
	k8sClient := testClient(t, source, pod)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodGet, "/sandboxes/agent/ports/web", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "awake" {
		t.Fatalf("expected proxied body, got %q", got)
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: "sandboxes", Name: "agent"}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Spec.DesiredState != computev1.SandboxDesiredStateRunning {
		t.Fatalf("expected port proxy to wake sandbox, got %s", updated.Spec.DesiredState)
	}
	if updated.Spec.Restore == nil || updated.Spec.Restore.Name != "agent-sleep" {
		t.Fatalf("expected port proxy wake to restore from sleep snapshot, got %#v", updated.Spec.Restore)
	}
	if updated.Status.LastActivityTime == nil {
		t.Fatal("expected port proxy wake to touch activity")
	}
}

func TestSandboxLogsStreamsWorkloadLogs(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status:     computev1.SandboxStatus{PodName: "sandbox-agent-running"},
	}
	k8sClient := testClient(t, source)
	streamer := &fakeLogStreamer{body: "hello\n"}
	server := New(k8sClient, "sandboxes", WithPodLogStreamer(streamer))

	res := request(t, server, http.MethodGet, "/sandboxes/agent/logs?tailLines=50&timestamps=true", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if got := res.Body.String(); got != "hello\n" {
		t.Fatalf("unexpected log body %q", got)
	}
	if got := res.Header().Get("Content-Type"); got != "text/plain; charset=utf-8" {
		t.Fatalf("unexpected content type %q", got)
	}
	if streamer.namespace != "sandboxes" || streamer.podName != "sandbox-agent-running" {
		t.Fatalf("unexpected log target namespace=%q pod=%q", streamer.namespace, streamer.podName)
	}
	if streamer.options.Container != "workload" {
		t.Fatalf("expected workload container, got %q", streamer.options.Container)
	}
	if streamer.options.TailLines == nil || *streamer.options.TailLines != 50 {
		t.Fatalf("expected tailLines 50, got %#v", streamer.options.TailLines)
	}
	if !streamer.options.Timestamps {
		t.Fatalf("expected timestamps option")
	}
}

func TestSandboxLogsSupportsKubernetesLogOptions(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	k8sClient := testClient(t, source)
	streamer := &fakeLogStreamer{body: "toolbox\n"}
	server := New(k8sClient, "sandboxes", WithPodLogStreamer(streamer))

	res := request(t, server, http.MethodGet, "/sandboxes/agent/logs?container=toolbox&follow=true&previous=true&sinceSeconds=30&limitBytes=4096", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	if streamer.podName != "sandbox-agent" {
		t.Fatalf("expected rendered pod name fallback, got %q", streamer.podName)
	}
	if streamer.options.Container != "toolbox" || !streamer.options.Follow || !streamer.options.Previous {
		t.Fatalf("unexpected options: %#v", streamer.options)
	}
	if streamer.options.SinceSeconds == nil || *streamer.options.SinceSeconds != 30 {
		t.Fatalf("expected sinceSeconds 30, got %#v", streamer.options.SinceSeconds)
	}
	if streamer.options.LimitBytes == nil || *streamer.options.LimitBytes != 4096 {
		t.Fatalf("expected limitBytes 4096, got %#v", streamer.options.LimitBytes)
	}
}

func TestSandboxLogsRejectsInvalidQuery(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	k8sClient := testClient(t, source)
	streamer := &fakeLogStreamer{body: "hello\n"}
	server := New(k8sClient, "sandboxes", WithPodLogStreamer(streamer))

	res := request(t, server, http.MethodGet, "/sandboxes/agent/logs?tailLines=-1", "")
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", res.Code, res.Body.String())
	}
	if streamer.options != nil {
		t.Fatalf("expected log streamer not to be called, got %#v", streamer.options)
	}
}

func TestExecWakesStoppedSandbox(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
		},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodPost, "/sandboxes/agent/exec", `{"command":["echo","ok"]}`)
	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", res.Code, res.Body.String())
	}
	var updated computev1.Sandbox
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Name: "agent", Namespace: "sandboxes"}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Spec.DesiredState != computev1.SandboxDesiredStateRunning {
		t.Fatalf("expected exec to wake sandbox, got %s", updated.Spec.DesiredState)
	}
	if updated.Status.LastActivityTime == nil {
		t.Fatal("expected exec wake to update last activity time")
	}
}

func TestStartWakesFromSleepSnapshot(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
			StopPolicy: computev1.SandboxStopPolicySpec{
				SnapshotBeforeStop: true,
				SnapshotName:       "agent-sleep-old",
				AutoStopMinutes:    60,
			},
		},
		Status: computev1.SandboxStatus{SleepSnapshotName: "agent-sleep-new"},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodPost, "/sandboxes/agent:start", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Name: "agent", Namespace: "sandboxes"}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Spec.DesiredState != computev1.SandboxDesiredStateRunning {
		t.Fatalf("expected start to wake sandbox, got %s", updated.Spec.DesiredState)
	}
	if updated.Spec.Restore == nil || updated.Spec.Restore.Name != "agent-sleep-new" {
		t.Fatalf("expected wake restore from sleep snapshot, got %#v", updated.Spec.Restore)
	}
	if updated.Spec.StopPolicy.SnapshotName != "" {
		t.Fatalf("expected stale sleep snapshot name to be cleared, got %#v", updated.Spec.StopPolicy)
	}
	if updated.Status.LastActivityTime == nil {
		t.Fatal("expected start wake to update last activity time")
	}
}

func TestSSHWakesStoppedSandbox(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
		},
		Status: computev1.SandboxStatus{SleepSnapshotName: "agent-sleep-new"},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodGet, "/sandboxes/agent/ssh", "")
	if res.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", res.Code, res.Body.String())
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(t.Context(), client.ObjectKey{Name: "agent", Namespace: "sandboxes"}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Spec.DesiredState != computev1.SandboxDesiredStateRunning {
		t.Fatalf("expected ssh to wake sandbox, got %s", updated.Spec.DesiredState)
	}
	if updated.Spec.Restore == nil || updated.Spec.Restore.Name != "agent-sleep-new" {
		t.Fatalf("expected ssh wake restore from sleep snapshot, got %#v", updated.Spec.Restore)
	}
	if updated.Status.LastActivityTime == nil {
		t.Fatal("expected ssh wake to update last activity time")
	}
}

func TestSSHReturnsKubernetesFallbackForRunningSandbox(t *testing.T) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Access: computev1.SandboxAccessSpec{
				SSHEnabled: true,
			},
		},
		Status: computev1.SandboxStatus{Phase: computev1.SandboxPhaseRunning},
	}
	k8sClient := testClient(t, source)
	server := New(k8sClient, "sandboxes")

	res := request(t, server, http.MethodGet, "/sandboxes/agent/ssh", "")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", res.Code, res.Body.String())
	}
	var ssh SSHResponse
	decode(t, res, &ssh)
	if !ssh.Enabled || ssh.Host != "agent.sandbox.tailnet" || ssh.Port != 22 || ssh.Username != "daytona" {
		t.Fatalf("unexpected ssh response %#v", ssh)
	}
}

func testClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := computev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}, &computev1.SandboxSnapshot{}).
		WithObjects(objects...).
		Build()
}

type fakeLogStreamer struct {
	namespace string
	podName   string
	options   *corev1.PodLogOptions
	body      string
	err       error
}

type fakePodExecutor struct {
	namespace     string
	podName       string
	containerName string
	request       ExecRequest
	response      ExecResponse
	err           error
}

func (f *fakePodExecutor) Exec(_ context.Context, namespace string, podName string, containerName string, req ExecRequest) (ExecResponse, error) {
	f.namespace = namespace
	f.podName = podName
	f.containerName = containerName
	f.request = req
	if f.err != nil {
		return ExecResponse{}, f.err
	}
	return f.response, nil
}

func (f *fakeLogStreamer) StreamPodLogs(_ context.Context, namespace string, podName string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	f.namespace = namespace
	f.podName = podName
	if options != nil {
		copied := *options
		f.options = &copied
	}
	if f.err != nil {
		return nil, f.err
	}
	return io.NopCloser(strings.NewReader(f.body)), nil
}

func request(t *testing.T, handler http.Handler, method string, path string, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decode(t *testing.T, res *httptest.ResponseRecorder, target any) {
	t.Helper()
	if err := json.NewDecoder(res.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}
