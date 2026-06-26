package controller

import (
	"context"
	"testing"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/gke"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestSandboxSnapshotReconcilerCreatesLocalRunscRequest(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status:     computev1.SandboxStatus{PodName: render.PodName(&computev1.Sandbox{ObjectMeta: metav1.ObjectMeta{Name: "agent"}})},
	}
	sourcePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: render.PodName(source), Namespace: source.Namespace},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
	}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderLocalRunsc,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
			Local: computev1.LocalRunscProviderSpec{
				Storage: computev1.LocalRunscStorageSpec{Mode: "filesystem", Path: "/snapshots"},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.SandboxSnapshot{}, &computev1.LocalRunscSnapshot{}).
		WithObjects(source, sourcePod, snapshot).
		Build()
	reconciler := &SandboxSnapshotReconciler{Client: k8sClient, Scheme: scheme}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace},
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	_, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	var request computev1.LocalRunscSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: gke.ObjectName("lrs", "warm"), Namespace: "default"}, &request); err != nil {
		t.Fatal(err)
	}
	if request.Spec.NodeName != "node-a" || request.Spec.SourcePodName != sourcePod.Name {
		t.Fatalf("unexpected local request spec: %#v", request.Spec)
	}
	if request.Spec.SourceContainerName != render.WorkloadContainerName {
		t.Fatalf("expected workload source container, got %q", request.Spec.SourceContainerName)
	}
}

func TestSandboxSnapshotReconcilerMirrorsReadyLocalRunscRequest(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
		Status:     computev1.SandboxStatus{PodName: "sandbox-agent"},
	}
	sourcePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-agent", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
	}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderLocalRunsc,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
		},
	}
	local := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: gke.ObjectName("lrs", "warm"), Namespace: "default"},
		Status: computev1.LocalRunscSnapshotStatus{
			Phase:      computev1.LocalRunscSnapshotPhaseReady,
			StorageRef: "/snapshots/warm",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.SandboxSnapshot{}, &computev1.LocalRunscSnapshot{}).
		WithObjects(source, sourcePod, snapshot, local).
		Build()
	reconciler := &SandboxSnapshotReconciler{Client: k8sClient, Scheme: scheme}

	req := ctrl.Request{
		NamespacedName: types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace},
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	_, err := reconciler.Reconcile(ctx, req)
	if err != nil {
		t.Fatal(err)
	}

	var updated computev1.SandboxSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxSnapshotPhaseReady {
		t.Fatalf("expected ready phase, got %s", updated.Status.Phase)
	}
	if updated.Status.ProviderObjectName != local.Name || updated.Status.StorageRef != "/snapshots/warm" {
		t.Fatalf("expected local provider refs, got %#v", updated.Status)
	}
}

func TestSandboxSnapshotReconcilerRecordsTemplateCompatibility(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	template := &computev1.SandboxTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "ubuntu", Namespace: "default"},
		Spec: computev1.SandboxTemplateSpec{
			Template: computev1.SandboxSpec{Image: "ubuntu:24.04"},
		},
		Status: computev1.SandboxTemplateStatus{CompatibilityHash: "template-hash"},
	}
	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			TemplateName: "ubuntu",
			Image:        "ubuntu:24.04",
		},
		Status: computev1.SandboxStatus{PodName: "sandbox-agent"},
	}
	sourcePod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "sandbox-agent", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
	}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderLocalRunsc,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.SandboxSnapshot{}, &computev1.LocalRunscSnapshot{}).
		WithObjects(template, source, sourcePod, snapshot).
		Build()
	reconciler := &SandboxSnapshotReconciler{Client: k8sClient, Scheme: scheme}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	var updated computev1.SandboxSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.TemplateName != "ubuntu" || updated.Status.CompatibilityHash != "template-hash" {
		t.Fatalf("expected template compatibility in status, got %#v", updated.Status)
	}
}

func TestSandboxSnapshotReconcilerRejectsPVCBackedSource(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	source := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Volumes: []computev1.SandboxVolumeSpec{{
				Name:      "state",
				MountPath: "/state",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "agent-state",
				},
			}},
		},
	}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Spec: computev1.SandboxSnapshotSpec{
			Source: computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.SandboxSnapshot{}).
		WithObjects(source, snapshot).
		Build()
	reconciler := &SandboxSnapshotReconciler{Client: k8sClient, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	var updated computev1.SandboxSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxSnapshotPhaseFailed {
		t.Fatalf("expected failed snapshot, got %#v", updated.Status)
	}
}

func TestSandboxSnapshotReconcilerFailsAndCleansStaleSnapshot(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	old := metav1.NewTime(now.Add(-2 * time.Hour))
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "warm",
			Namespace:         "default",
			CreationTimestamp: old,
		},
		Spec: computev1.SandboxSnapshotSpec{
			Source: computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
		},
		Status: computev1.SandboxSnapshotStatus{
			Phase: computev1.SandboxSnapshotPhaseTriggering,
			Conditions: []metav1.Condition{{
				Type:               "Ready",
				Status:             metav1.ConditionFalse,
				Reason:             "Waiting",
				Message:            "waiting",
				LastTransitionTime: old,
			}},
		},
	}
	controllerutil.AddFinalizer(snapshot, computev1.SandboxSnapshotFinalizer)
	local := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: gke.ObjectName("lrs", snapshot.Name), Namespace: snapshot.Namespace},
	}
	policy := &unstructured.Unstructured{}
	policy.SetAPIVersion(gke.APIVersion)
	policy.SetKind(gke.PolicyKind)
	policy.SetName(gke.ObjectName("psp", snapshot.Name))
	policy.SetNamespace(snapshot.Namespace)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.SandboxSnapshot{}).
		WithObjects(snapshot, local, policy).
		Build()
	reconciler := &SandboxSnapshotReconciler{
		Client:       k8sClient,
		Scheme:       scheme,
		StaleTimeout: time.Hour,
		Now:          func() time.Time { return now },
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: snapshot.Name, Namespace: snapshot.Namespace}}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}

	var updated computev1.SandboxSnapshot
	if err := k8sClient.Get(ctx, req.NamespacedName, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxSnapshotPhaseFailed || updated.Status.Error == "" {
		t.Fatalf("expected stale snapshot to fail with error, got %#v", updated.Status)
	}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: local.Name, Namespace: local.Namespace}, &computev1.LocalRunscSnapshot{}); !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale local request to be deleted, got %v", err)
	}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: policy.GetName(), Namespace: policy.GetNamespace()}, policy); !apierrors.IsNotFound(err) {
		t.Fatalf("expected stale policy to be deleted, got %v", err)
	}
}
