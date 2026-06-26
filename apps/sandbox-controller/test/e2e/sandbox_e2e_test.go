//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/gke"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/remotecommand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestSandboxLifecycleE2E(t *testing.T) {
	if os.Getenv("DAYTONA_E2E") != "1" {
		t.Skip("set DAYTONA_E2E=1 to run cluster lifecycle tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	k8sClient := e2eClient(t)
	namespace := e2eNamespace(t, k8sClient, ctx)
	name := uniqueName("e2e-agent")
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: e2eLabels(t)},
		Spec: computev1.SandboxSpec{
			Image:   "ubuntu:24.04",
			Command: []string{"/bin/sh", "-lc"},
			Args:    []string{"sleep infinity"},
			Ports:   []computev1.SandboxPort{{Name: "http", Port: 8080}},
		},
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, sandbox)
	})
	if err := k8sClient.Create(ctx, sandbox); err != nil {
		t.Fatal(err)
	}

	waitFor(ctx, t, func() (bool, error) {
		var current computev1.Sandbox
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &current); err != nil {
			return false, err
		}
		return current.Status.PodName != "" && current.Status.ServiceName != "", nil
	})

	var current computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &current); err != nil {
		t.Fatal(err)
	}
	current.Spec.DesiredState = computev1.SandboxDesiredStateStopped
	if err := k8sClient.Update(ctx, &current); err != nil {
		t.Fatal(err)
	}
	waitFor(ctx, t, func() (bool, error) {
		var stopped computev1.Sandbox
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &stopped); err != nil {
			return false, err
		}
		return stopped.Status.Phase == computev1.SandboxPhaseStopped, nil
	})
}

func TestLocalRunscSnapshotE2E(t *testing.T) {
	if os.Getenv("DAYTONA_LOCAL_RUNSC_E2E") != "1" {
		t.Skip("set DAYTONA_LOCAL_RUNSC_E2E=1 to run local VM PodSnapshot shim tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	k8sClient := e2eClient(t)
	namespace := e2eNamespace(t, k8sClient, ctx)
	sourceName := uniqueName("local-source")
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: sourceName, Namespace: namespace, Labels: e2eLabels(t)},
		Spec: computev1.SandboxSpec{
			Image:   "ubuntu:24.04",
			Command: []string{"/bin/sh", "-lc"},
			Args:    []string{"sleep infinity"},
		},
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, source)
	})
	if err := k8sClient.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	waitFor(ctx, t, func() (bool, error) {
		var current computev1.Sandbox
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: namespace}, &current); err != nil {
			return false, err
		}
		return current.Status.Phase == computev1.SandboxPhaseRunning && current.Status.PodName != "", nil
	})

	snapshotName := uniqueName("local-runsc")
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: snapshotName, Namespace: namespace, Labels: e2eLabels(t)},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderGKEPodSnapshot,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: sourceName},
			GKE: computev1.GKEPodSnapshotSpec{
				StorageConfigName: envDefault("DAYTONA_LOCAL_PODSNAPSHOT_STORAGE_CONFIG", "local-minio"),
				PostCheckpoint:    computev1.PostCheckpointResume,
			},
		},
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, snapshot)
	})
	if err := k8sClient.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	waitFor(ctx, t, func() (bool, error) {
		var current computev1.SandboxSnapshot
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: namespace}, &current); err != nil {
			return false, err
		}
		if current.Status.Phase == computev1.SandboxSnapshotPhaseFailed {
			return false, fmt.Errorf("snapshot failed: %s", current.Status.Error)
		}
		return current.Status.Phase == computev1.SandboxSnapshotPhaseReady && current.Status.StorageRef != "", nil
	})
}

func TestGKEPodSnapshotRestoreE2E(t *testing.T) {
	if os.Getenv("DAYTONA_GKE_E2E") != "1" {
		t.Skip("set DAYTONA_GKE_E2E=1 to run GKE PodSnapshot restore tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	k8sClient := e2eClient(t)
	namespace := e2eNamespace(t, k8sClient, ctx)
	requireCurrentContext(t, stgContextName)
	requireObject(t, ctx, k8sClient, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "podsnapshots.podsnapshot.gke.io")
	requireObject(t, ctx, k8sClient, "node.k8s.io/v1", "RuntimeClass", "", "gvisor")
	requireGvisorNodePool(t, ctx, k8sClient)

	sourceName := uniqueName("gke-source")
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: sourceName, Namespace: namespace, Labels: e2eLabels(t)},
		Spec: computev1.SandboxSpec{
			Image:   "ubuntu:24.04",
			Command: []string{"/bin/sh", "-lc"},
			Args:    []string{"sleep infinity"},
			Ports:   []computev1.SandboxPort{{Name: "http", Port: 8080}},
		},
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, source)
	})
	if err := k8sClient.Create(ctx, source); err != nil {
		t.Fatal(err)
	}
	waitFor(ctx, t, func() (bool, error) {
		var current computev1.Sandbox
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: namespace}, &current); err != nil {
			return false, err
		}
		if current.Status.Phase == computev1.SandboxPhaseFailed {
			return false, fmt.Errorf("source sandbox failed")
		}
		return current.Status.Phase == computev1.SandboxPhaseRunning && current.Status.PodName != "", nil
	})
	var sourceReady computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: namespace}, &sourceReady); err != nil {
		t.Fatal(err)
	}
	assertExternalConnectionCanOpenAndClose(ctx, t, namespace, sourceReady.Status.PodName, "closed-before-snapshot")

	storageConfigName := uniqueName("e2e-storage")
	storageConfig := gkeE2EStorageConfig(storageConfigName, requireEnv(t, "DAYTONA_GKE_STORAGE_BUCKET"), requireEnv(t, "DAYTONA_GKE_STORAGE_PREFIX")+"/"+e2eRunID(t), e2eRunID(t))
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, storageConfig)
	})
	if err := k8sClient.Create(ctx, storageConfig); err != nil {
		t.Fatal(err)
	}

	snapshotName := uniqueName("gke-podsnapshot")
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: snapshotName, Namespace: namespace, Labels: e2eLabels(t)},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderGKEPodSnapshot,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: sourceName},
			GKE: computev1.GKEPodSnapshotSpec{
				StorageConfigName: storageConfigName,
				PostCheckpoint:    computev1.PostCheckpointResume,
			},
		},
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, snapshot)
	})
	if err := k8sClient.Create(ctx, snapshot); err != nil {
		t.Fatal(err)
	}

	var readySnapshot computev1.SandboxSnapshot
	waitFor(ctx, t, func() (bool, error) {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: snapshotName, Namespace: namespace}, &readySnapshot); err != nil {
			return false, err
		}
		if readySnapshot.Status.Phase == computev1.SandboxSnapshotPhaseFailed {
			return false, fmt.Errorf("snapshot failed: %s", readySnapshot.Status.Error)
		}
		return readySnapshot.Status.Phase == computev1.SandboxSnapshotPhaseReady && readySnapshot.Status.ProviderObjectName != "", nil
	})

	var sourceCurrent computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: namespace}, &sourceCurrent); err != nil {
		t.Fatal(err)
	}
	forkName := uniqueName("gke-restore")
	forkSpec := sourceCurrent.Spec.DeepCopy()
	forkSpec.DesiredState = computev1.SandboxDesiredStateRunning
	forkSpec.Restore = &computev1.SandboxSnapshotRestoreRef{
		Name:               readySnapshot.Name,
		Provider:           computev1.SnapshotProviderGKEPodSnapshot,
		ProviderObjectName: readySnapshot.Status.ProviderObjectName,
	}
	fork := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: forkName, Namespace: namespace, Labels: e2eLabels(t)},
		Spec:       forkSpec,
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()
		_ = k8sClient.Delete(cleanupCtx, fork)
	})
	if err := k8sClient.Create(ctx, fork); err != nil {
		t.Fatal(err)
	}

	var restored computev1.Sandbox
	waitFor(ctx, t, func() (bool, error) {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: forkName, Namespace: namespace}, &restored); err != nil {
			return false, err
		}
		if restored.Status.Phase == computev1.SandboxPhaseFailed {
			return false, fmt.Errorf("restored sandbox failed")
		}
		return restored.Status.Phase == computev1.SandboxPhaseRunning && restored.Status.PodName != "", nil
	})

	var restoredPod corev1.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: restored.Status.PodName, Namespace: namespace}, &restoredPod); err != nil {
		t.Fatal(err)
	}
	if got := restoredPod.Annotations[computev1.GKERestoreSnapshotAnnotation]; got != readySnapshot.Status.ProviderObjectName {
		t.Fatalf("expected restored Pod annotation %s=%q, got %q", computev1.GKERestoreSnapshotAnnotation, readySnapshot.Status.ProviderObjectName, got)
	}
	if got := restoredPod.Spec.RuntimeClassName; got == nil || *got != render.RuntimeClassName(&restored) {
		t.Fatalf("expected restored Pod runtime class %q, got %#v", render.RuntimeClassName(&restored), got)
	}
	assertExternalConnectionCanOpenAndClose(ctx, t, namespace, restored.Status.PodName, "fresh-after-restore")
}

func gkeE2EStorageConfig(name string, bucket string, path string, runID string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"snapshotStorageConfig": map[string]any{
					"gcs": map[string]any{
						"bucket":      bucket,
						"path":        path,
						"tokenSource": "podKSA",
					},
				},
			},
		},
	}
	obj.SetAPIVersion(gke.APIVersion)
	obj.SetKind(gke.StorageConfigKind)
	obj.SetName(name)
	obj.SetLabels(map[string]string{
		e2eLabelKey:      "true",
		e2eRunIDLabelKey: runID,
	})
	return obj
}

func e2eClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := computev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8sClient, err := client.New(ctrl.GetConfigOrDie(), client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	return k8sClient
}

func e2eNamespace(t *testing.T, k8sClient client.Client, ctx context.Context) string {
	t.Helper()
	namespace := envDefault("DAYTONA_E2E_NAMESPACE", "daytona-sandbox-e2e")
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace, Labels: map[string]string{e2eLabelKey: "true"}}}
	if err := k8sClient.Create(ctx, ns); err != nil && !apierrors.IsAlreadyExists(err) {
		t.Fatal(err)
	}
	return namespace
}

func waitFor(ctx context.Context, t *testing.T, check func() (bool, error)) {
	t.Helper()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		ok, err := check()
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-ticker.C:
		}
	}
}

func assertExternalConnectionCanOpenAndClose(ctx context.Context, t *testing.T, namespace string, podName string, marker string) {
	t.Helper()
	stdout, stderr, err := execInWorkload(ctx, namespace, podName, []string{
		"/bin/bash",
		"-lc",
		fmt.Sprintf("set -euo pipefail; exec 3<>/dev/tcp/kubernetes.default.svc/443; exec 3>&-; echo %s", marker),
	})
	if err != nil {
		t.Fatalf("external TCP open/close probe failed: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, marker) {
		t.Fatalf("expected external TCP probe marker %q in stdout %q; stderr: %s", marker, stdout, stderr)
	}
}

func execInWorkload(ctx context.Context, namespace string, podName string, command []string) (string, string, error) {
	config := ctrl.GetConfigOrDie()
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return "", "", err
	}
	req := clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: render.WorkloadContainerName,
			Command:   command,
			Stdout:    true,
			Stderr:    true,
		}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
	if err != nil {
		return "", "", err
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	err = executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: &stdout,
		Stderr: &stderr,
	})
	return stdout.String(), stderr.String(), err
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func envDefault(name string, fallback string) string {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	return value
}
