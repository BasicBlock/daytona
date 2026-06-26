package localrunsc

import (
	"context"
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSnapshotReconcilerCheckpointsAssignedNodeRequest(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := computev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	snapshot := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "sandboxes"},
		Spec: computev1.LocalRunscSnapshotSpec{
			SandboxName:         "agent",
			SourcePodName:       "sandbox-agent",
			SourceContainerName: "workload",
			NodeName:            "node-a",
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-agent", Namespace: "sandboxes"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{{
				Name:        "workload",
				ContainerID: "containerd://runtime-container-id",
			}},
		},
	}
	runner := &recordingRunner{}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.LocalRunscSnapshot{}).
		WithObjects(snapshot, pod).
		Build()

	reconciler := &SnapshotReconciler{
		Client:   k8sClient,
		Runtime:  NewRuntime("runsc", t.TempDir(), runner),
		NodeName: "node-a",
	}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "warm", Namespace: "sandboxes"}}

	result, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if result.Requeue {
		t.Fatal("expected finalizer reconcile not to request requeue")
	}

	result, err = reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Requeue {
		t.Fatal("expected running phase reconcile to request requeue")
	}

	var running computev1.LocalRunscSnapshot
	if err := k8sClient.Get(ctx, req.NamespacedName, &running); err != nil {
		t.Fatal(err)
	}
	if running.Status.Phase != computev1.LocalRunscSnapshotPhaseRunning {
		t.Fatalf("expected running phase, got %s", running.Status.Phase)
	}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	var ready computev1.LocalRunscSnapshot
	if err := k8sClient.Get(ctx, req.NamespacedName, &ready); err != nil {
		t.Fatal(err)
	}
	if ready.Status.Phase != computev1.LocalRunscSnapshotPhaseReady || ready.Status.StorageRef == "" {
		t.Fatalf("expected ready snapshot with storage ref, got %#v", ready.Status)
	}
	if got := runner.args[len(runner.args)-1]; got != "runtime-container-id" {
		t.Fatalf("expected stripped runtime container id, got %q", got)
	}
}

func TestSnapshotReconcilerCleansArtifactsOnDelete(t *testing.T) {
	ctx := context.Background()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := computev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	now := metav1.Now()
	snapshot := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "warm",
			Namespace:         "sandboxes",
			Finalizers:        []string{computev1.LocalRunscSnapshotFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: computev1.LocalRunscSnapshotSpec{
			NodeName: "node-a",
			Storage: computev1.LocalRunscStorageSpec{
				Mode:   StorageModeS3,
				Bucket: "daytona-local",
			},
		},
		Status: computev1.LocalRunscSnapshotStatus{
			StorageRef: "s3://daytona-local/snapshots/sandboxes/warm",
		},
	}
	store := &recordingStore{}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.LocalRunscSnapshot{}).
		WithObjects(snapshot).
		Build()

	reconciler := &SnapshotReconciler{
		Client:   k8sClient,
		Runtime:  NewRuntime("runsc", t.TempDir(), &recordingRunner{}),
		NodeName: "node-a",
	}
	reconciler.Runtime.Store = store
	if _, err := reconciler.reconcileDelete(ctx, snapshot); err != nil {
		t.Fatal(err)
	}
	if !store.deleted {
		t.Fatal("expected object store cleanup")
	}
}
