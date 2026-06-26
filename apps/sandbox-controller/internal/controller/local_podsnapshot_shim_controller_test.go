package controller

import (
	"context"
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/gke"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestLocalPodSnapshotShimCreatesLocalRunscRequest(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	source, snapshot, trigger, policy, storageConfig, pod := localShimObjects()

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.LocalRunscSnapshot{}, gkeManualTrigger(), gkePodSnapshot()).
		WithObjects(source, snapshot, trigger, policy, storageConfig, pod).
		Build()
	reconciler := &LocalPodSnapshotShimReconciler{Client: k8sClient, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: trigger.GetName(), Namespace: trigger.GetNamespace()}}); err != nil {
		t.Fatal(err)
	}

	var request computev1.LocalRunscSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: gke.ObjectName("lrs", trigger.GetName()), Namespace: trigger.GetNamespace()}, &request); err != nil {
		t.Fatal(err)
	}
	if request.Spec.NodeName != "node-a" || request.Spec.SourcePodName != pod.Name {
		t.Fatalf("unexpected local request spec: %#v", request.Spec)
	}
	if request.Spec.Storage.Mode != "s3" || request.Spec.Storage.Bucket != "daytona-local" || request.Spec.Storage.Prefix != "snapshots" {
		t.Fatalf("expected local MinIO storage from GKE-shaped storage config, got %#v", request.Spec.Storage)
	}
}

func TestLocalPodSnapshotShimMirrorsReadyRequestToPodSnapshot(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	source, snapshot, trigger, policy, storageConfig, pod := localShimObjects()
	local := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: gke.ObjectName("lrs", trigger.GetName()), Namespace: trigger.GetNamespace()},
		Status: computev1.LocalRunscSnapshotStatus{
			Phase:      computev1.LocalRunscSnapshotPhaseReady,
			StorageRef: "s3://daytona-local/snapshots/sandboxes/warm",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.LocalRunscSnapshot{}, gkeManualTrigger(), gkePodSnapshot()).
		WithObjects(source, snapshot, trigger, policy, storageConfig, pod, local).
		Build()
	reconciler := &LocalPodSnapshotShimReconciler{Client: k8sClient, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: trigger.GetName(), Namespace: trigger.GetNamespace()}}); err != nil {
		t.Fatal(err)
	}

	podSnapshot := &unstructured.Unstructured{}
	podSnapshot.SetAPIVersion(gke.APIVersion)
	podSnapshot.SetKind(gke.PodSnapshotKind)
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: gke.ObjectName("ps", trigger.GetName()), Namespace: trigger.GetNamespace()}, podSnapshot); err != nil {
		t.Fatal(err)
	}
	if !gke.IsReady(podSnapshot) {
		t.Fatalf("expected local PodSnapshot to be ready: %#v", podSnapshot.Object["status"])
	}
	storageRef, _, _ := unstructured.NestedString(podSnapshot.Object, "status", "artifactStorageRef")
	if storageRef != local.Status.StorageRef {
		t.Fatalf("expected artifact ref %q, got %q", local.Status.StorageRef, storageRef)
	}
}

func localShimObjects() (*computev1.Sandbox, *computev1.SandboxSnapshot, *unstructured.Unstructured, *unstructured.Unstructured, *unstructured.Unstructured, *corev1.Pod) {
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status:     computev1.SandboxStatus{PodName: "sandbox-agent"},
	}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "sandboxes"},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderGKEPodSnapshot,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
			GKE: computev1.GKEPodSnapshotSpec{
				Storage: computev1.GKEPodSnapshotStorage{
					Bucket: "daytona-local",
					Path:   "snapshots",
				},
			},
		},
	}
	policy := gke.Policy(snapshot, source)
	storageConfig := gke.StorageConfig(snapshot)
	trigger := gke.ManualTrigger(snapshot, source)
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      render.PodName(source),
			Namespace: source.Namespace,
			Labels: map[string]string{
				computev1.LabelSandboxName: source.Name,
			},
		},
		Spec: corev1.PodSpec{NodeName: "node-a"},
	}
	return source, snapshot, trigger, policy, storageConfig, pod
}

func gkePodSnapshot() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(gke.APIVersion)
	obj.SetKind(gke.PodSnapshotKind)
	return obj
}
