package render

import (
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPodDefaultsToGvisorAndToolboxSidecar(t *testing.T) {
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-1", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Ports: []computev1.SandboxPort{{
				Name: "http",
				Port: 8080,
			}},
		},
	}

	pod, specHash, err := Pod(sandbox, "")
	if err != nil {
		t.Fatal(err)
	}

	if specHash == "" {
		t.Fatal("expected spec hash")
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != DefaultRuntimeClassName {
		t.Fatalf("expected runtimeClassName %q, got %#v", DefaultRuntimeClassName, pod.Spec.RuntimeClassName)
	}
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected workload and toolbox containers, got %d", len(pod.Spec.Containers))
	}
	if pod.Spec.Containers[1].Name != ToolboxContainerName {
		t.Fatalf("expected toolbox sidecar, got %s", pod.Spec.Containers[1].Name)
	}
	if pod.Spec.Containers[1].SecurityContext == nil || pod.Spec.Containers[1].SecurityContext.RunAsUser == nil || *pod.Spec.Containers[1].SecurityContext.RunAsUser != 0 {
		t.Fatalf("expected toolbox sidecar to run as root for namespace entry, got %#v", pod.Spec.Containers[1].SecurityContext)
	}
	if pod.Spec.Containers[0].Ports[0].Name != "http" {
		t.Fatalf("expected workload port to be named http, got %s", pod.Spec.Containers[0].Ports[0].Name)
	}
}

func TestPodAddsGKERestoreAnnotation(t *testing.T) {
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-2", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Restore: &computev1.SandboxSnapshotRestoreRef{
				Name:               "agent-2-warm",
				ProviderObjectName: "gke-podsnapshot-123",
			},
		},
	}

	pod, _, err := Pod(sandbox, "")
	if err != nil {
		t.Fatal(err)
	}

	if got := pod.Annotations[computev1.GKERestoreSnapshotAnnotation]; got != "gke-podsnapshot-123" {
		t.Fatalf("expected restore annotation, got %q", got)
	}
}

func TestPodAddsLocalRunscRestoreAnnotations(t *testing.T) {
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-2", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Restore: &computev1.SandboxSnapshotRestoreRef{
				Name:               "agent-2-warm",
				Provider:           computev1.SnapshotProviderLocalRunsc,
				ProviderObjectName: "lrs-agent-2-warm",
				StorageRef:         "/snapshots/agent-2-warm",
			},
		},
	}

	pod, _, err := Pod(sandbox, "")
	if err != nil {
		t.Fatal(err)
	}

	if got := pod.Annotations[computev1.LocalRunscRestoreSnapshotAnnotation]; got != "lrs-agent-2-warm" {
		t.Fatalf("expected local restore annotation, got %q", got)
	}
	if got := pod.Annotations[computev1.LocalRunscRestoreStorageRefAnnotation]; got != "/snapshots/agent-2-warm" {
		t.Fatalf("expected local storage annotation, got %q", got)
	}
	if got := pod.Annotations[computev1.GKERestoreSnapshotAnnotation]; got != "" {
		t.Fatalf("expected no GKE restore annotation, got %q", got)
	}
}

func TestCompatibilityHashIgnoresDesiredStateAndRestore(t *testing.T) {
	running := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			DesiredState: computev1.SandboxDesiredStateRunning,
			Image:        "ubuntu:24.04",
			Env: []corev1.EnvVar{
				{Name: "B", Value: "2"},
				{Name: "A", Value: "1"},
			},
			Restore: &computev1.SandboxSnapshotRestoreRef{
				Name:               "snap-a",
				ProviderObjectName: "provider-a",
			},
		},
	}
	stopped := running.DeepCopyObject().(*computev1.Sandbox)
	stopped.Spec.DesiredState = computev1.SandboxDesiredStateStopped
	stopped.Spec.Restore = &computev1.SandboxSnapshotRestoreRef{
		Name:               "snap-b",
		ProviderObjectName: "provider-b",
	}
	stopped.Spec.Env = []corev1.EnvVar{
		{Name: "A", Value: "1"},
		{Name: "B", Value: "2"},
	}

	runningHash, err := CompatibilityHash(running)
	if err != nil {
		t.Fatal(err)
	}
	stoppedHash, err := CompatibilityHash(stopped)
	if err != nil {
		t.Fatal(err)
	}

	if runningHash != stoppedHash {
		t.Fatalf("expected hashes to match, got %s and %s", runningHash, stoppedHash)
	}
}

func TestPodInjectsDopplerManagedSecret(t *testing.T) {
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Secrets: computev1.SandboxSecretsSpec{
				Provider:          "doppler",
				DopplerProject:    "daytona",
				DopplerConfig:     "stg",
				ManagedSecretName: "agent-doppler",
			},
		},
	}

	pod, _, err := Pod(sandbox, "")
	if err != nil {
		t.Fatal(err)
	}
	workload := pod.Spec.Containers[0]
	if len(workload.EnvFrom) != 1 || workload.EnvFrom[0].SecretRef == nil || workload.EnvFrom[0].SecretRef.Name != "agent-doppler" {
		t.Fatalf("expected workload envFrom doppler secret, got %#v", workload.EnvFrom)
	}
	toolbox := pod.Spec.Containers[1]
	foundProject := false
	foundConfig := false
	for _, env := range toolbox.Env {
		if env.Name == "DAYTONA_DOPPLER_PROJECT" && env.Value == "daytona" {
			foundProject = true
		}
		if env.Name == "DAYTONA_DOPPLER_CONFIG" && env.Value == "stg" {
			foundConfig = true
		}
	}
	if !foundProject || !foundConfig {
		t.Fatalf("expected toolbox Doppler metadata env, got %#v", toolbox.Env)
	}
}
