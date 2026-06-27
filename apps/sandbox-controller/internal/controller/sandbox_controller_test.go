package controller

import (
	"context"
	"fmt"
	"testing"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func TestSandboxReconcilerCreatesPodAndService(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Ports: []computev1.SandboxPort{{Name: "http", Port: 8080}},
			NetworkPolicy: computev1.SandboxNetworkPolicySpec{
				Enabled:     true,
				AllowDNS:    true,
				EgressCIDRs: []string{"10.0.0.0/8"},
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}}

	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil {
		t.Fatal(err)
	}

	var pod corev1.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &pod); err != nil {
		t.Fatal(err)
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != render.DefaultRuntimeClassName {
		t.Fatalf("expected gvisor runtime class, got %#v", pod.Spec.RuntimeClassName)
	}
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("expected workload plus toolbox containers, got %d", len(pod.Spec.Containers))
	}

	var service corev1.Service
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.ServiceName(sandbox), Namespace: sandbox.Namespace}, &service); err != nil {
		t.Fatal(err)
	}
	if len(service.Spec.Ports) != 2 {
		t.Fatalf("expected toolbox plus user service ports, got %d", len(service.Spec.Ports))
	}

	var policy networkingv1.NetworkPolicy
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.NetworkPolicyName(sandbox), Namespace: sandbox.Namespace}, &policy); err != nil {
		t.Fatal(err)
	}
	if len(policy.Spec.Egress) != 2 {
		t.Fatalf("expected DNS and CIDR egress rules, got %d", len(policy.Spec.Egress))
	}
}

func TestSandboxReconcilerDeletesPodWhenStopped(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, pod).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &corev1.Pod{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected pod to be deleted, got %v", err)
	}
}

func TestSandboxReconcilerReleasesFinalizerForStaleDeletingSandbox(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	deletingSince := metav1.NewTime(now.Add(-20 * time.Minute))
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "agent",
			Namespace:         "default",
			DeletionTimestamp: &deletingSince,
			Finalizers:        []string{computev1.SandboxFinalizer},
		},
		Spec: computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	service := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: render.ServiceName(sandbox), Namespace: sandbox.Namespace}}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, service).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if obj.GetName() == render.ServiceName(sandbox) {
					return fmt.Errorf("simulated dependent delete failure")
				}
				return c.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	reconciler := &SandboxReconciler{
		Client:             k8sClient,
		Scheme:             scheme,
		StaleDeleteTimeout: 10 * time.Minute,
		Now:                func() time.Time { return now },
	}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		t.Fatal(err)
	}
	if controllerutil.ContainsFinalizer(&updated, computev1.SandboxFinalizer) {
		t.Fatalf("expected stale deleting sandbox finalizer to be released, got %#v", updated.Finalizers)
	}
}

func TestSandboxReconcilerSnapshotsBeforeStop(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
			StopPolicy: computev1.SandboxStopPolicySpec{
				SnapshotBeforeStop: true,
				SnapshotName:       "agent-stop",
			},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, pod).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var snapshot computev1.SandboxSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "agent-stop", Namespace: "default"}, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Spec.Source.SandboxName != sandbox.Name {
		t.Fatalf("expected source sandbox %q, got %q", sandbox.Name, snapshot.Spec.Source.SandboxName)
	}

	var stillRunning corev1.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, &stillRunning); err != nil {
		t.Fatalf("expected pod to remain until snapshot is ready: %v", err)
	}
}

func TestSandboxReconcilerAutoStopsIdleSandbox(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	lastActivity := metav1.NewTime(now.Add(-2 * time.Minute))

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			StopPolicy: computev1.SandboxStopPolicySpec{
				AutoStopMinutes: 1,
			},
		},
		Status: computev1.SandboxStatus{
			Phase:            computev1.SandboxPhaseRunning,
			LastActivityTime: &lastActivity,
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, pod).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme, Now: func() time.Time { return now }}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Spec.DesiredState != computev1.SandboxDesiredStateStopped {
		t.Fatalf("expected idle sandbox to stop, got %s", updated.Spec.DesiredState)
	}
	if !updated.Spec.StopPolicy.SnapshotBeforeStop || updated.Spec.StopPolicy.SnapshotName != "agent-sleep-1782475200" {
		t.Fatalf("expected generated sleep snapshot policy, got %#v", updated.Spec.StopPolicy)
	}
}

func TestSandboxReconcilerCreatesPodForActiveAutoStopSandbox(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	lastActivity := metav1.NewTime(now)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			StopPolicy: computev1.SandboxStopPolicySpec{
				AutoStopMinutes: 1,
			},
		},
		Status: computev1.SandboxStatus{
			Phase:            computev1.SandboxPhaseRunning,
			LastActivityTime: &lastActivity,
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme, Now: func() time.Time { return now }}

	result, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter != time.Minute {
		t.Fatalf("expected idle requeue in one minute, got %s", result.RequeueAfter)
	}

	var pod corev1.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &pod); err != nil {
		t.Fatalf("expected active auto-stop sandbox to create pod: %v", err)
	}
}

func TestSandboxReconcilerDoesNotAutoStopWhileWaking(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	now := time.Date(2026, 6, 26, 12, 0, 0, 0, time.UTC)
	lastActivity := metav1.NewTime(now.Add(-2 * time.Minute))

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			DesiredState: computev1.SandboxDesiredStateRunning,
			Image:        "ubuntu:24.04",
			Restore:      &computev1.SandboxSnapshotRestoreRef{Name: "agent-sleep"},
			StopPolicy: computev1.SandboxStopPolicySpec{
				SnapshotBeforeStop: true,
				AutoStopMinutes:    1,
				Provider:           computev1.SnapshotProviderGKEPodSnapshot,
			},
		},
		Status: computev1.SandboxStatus{
			Phase:            computev1.SandboxPhaseStopped,
			LastActivityTime: &lastActivity,
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-sleep", Namespace: "default"},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderGKEPodSnapshot,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: "agent"},
		},
		Status: computev1.SandboxSnapshotStatus{
			Phase:              computev1.SandboxSnapshotPhaseReady,
			ProviderObjectName: "podsnapshot-a",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, snapshot).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme, Now: func() time.Time { return now }}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var pod corev1.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &pod); err != nil {
		t.Fatalf("expected waking sandbox to create restored pod: %v", err)
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Spec.DesiredState != computev1.SandboxDesiredStateRunning {
		t.Fatalf("expected waking sandbox to remain desired Running, got %s", updated.Spec.DesiredState)
	}
}

func TestSandboxReconcilerRecordsSleepSnapshotWhenStopped(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image:        "ubuntu:24.04",
			DesiredState: computev1.SandboxDesiredStateStopped,
			StopPolicy: computev1.SandboxStopPolicySpec{
				SnapshotBeforeStop: true,
				SnapshotName:       "agent-sleep-1",
			},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}}
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "agent-sleep-1", Namespace: "default"},
		Status: computev1.SandboxSnapshotStatus{
			Phase:              computev1.SandboxSnapshotPhaseReady,
			ProviderObjectName: "podsnapshot-a",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, pod, snapshot).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxPhaseStopped || updated.Status.SleepSnapshotName != "agent-sleep-1" {
		t.Fatalf("expected stopped sleep snapshot status, got %#v", updated.Status)
	}
}

func TestSandboxReconcilerBlocksRestoreUntilSnapshotReady(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "fork", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Restore: &computev1.SandboxSnapshotRestoreRef{
				Name: "warm",
			},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Status: computev1.SandboxSnapshotStatus{
			Phase: computev1.SandboxSnapshotPhaseTriggering,
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, snapshot).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	err = k8sClient.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &corev1.Pod{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected pod not to be created before snapshot is ready, got %v", err)
	}
}

func TestSandboxReconcilerRejectsIncompatibleSnapshot(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "fork", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Restore: &computev1.SandboxSnapshotRestoreRef{
				Name: "warm",
			},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Status: computev1.SandboxSnapshotStatus{
			Phase:              computev1.SandboxSnapshotPhaseReady,
			ProviderObjectName: "podsnapshot-a",
			CompatibilityHash:  "not-the-sandbox-hash",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, snapshot).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxPhaseFailed {
		t.Fatalf("expected failed phase, got %s", updated.Status.Phase)
	}
}

func TestSandboxReconcilerRejectsSnapshotTemplateMismatch(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "fork", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			TemplateName: "other",
			Image:        "ubuntu:24.04",
			Restore:      &computev1.SandboxSnapshotRestoreRef{Name: "warm"},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Status: computev1.SandboxSnapshotStatus{
			Phase:              computev1.SandboxSnapshotPhaseReady,
			ProviderObjectName: "podsnapshot-a",
			TemplateName:       "ubuntu",
			CompatibilityHash:  "template-hash",
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox, snapshot).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}}); err != nil {
		t.Fatal(err)
	}
	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxPhaseFailed {
		t.Fatalf("expected failed phase, got %s", updated.Status.Phase)
	}
}

func TestSandboxReconcilerAllowsMatchingTemplateRestore(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	templateSpec := computev1.SandboxSpec{Image: "ubuntu:24.04"}
	templateSandbox := &computev1.Sandbox{Spec: templateSpec}
	templateHash, err := render.CompatibilityHash(templateSandbox)
	if err != nil {
		t.Fatal(err)
	}
	template := &computev1.SandboxTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "ubuntu", Namespace: "default"},
		Spec:       computev1.SandboxTemplateSpec{Template: templateSpec},
		Status:     computev1.SandboxTemplateStatus{CompatibilityHash: templateHash},
	}
	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "fork", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			TemplateName: "ubuntu",
			Image:        "ubuntu:24.04",
			Restore:      &computev1.SandboxSnapshotRestoreRef{Name: "warm"},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "default"},
		Status: computev1.SandboxSnapshotStatus{
			Phase:              computev1.SandboxSnapshotPhaseReady,
			ProviderObjectName: "podsnapshot-a",
			TemplateName:       "ubuntu",
			CompatibilityHash:  templateHash,
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(template, sandbox, snapshot).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}}); err != nil {
		t.Fatal(err)
	}
	var pod corev1.Pod
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &pod); err != nil {
		t.Fatal(err)
	}
}

func TestSandboxReconcilerRejectsPVCRestore(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "fork", Namespace: "default"},
		Spec: computev1.SandboxSpec{
			Image: "ubuntu:24.04",
			Volumes: []computev1.SandboxVolumeSpec{{
				Name:      "state",
				MountPath: "/state",
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "state",
				},
			}},
			Restore: &computev1.SandboxSnapshotRestoreRef{Name: "warm"},
		},
	}
	controllerutil.AddFinalizer(sandbox, computev1.SandboxFinalizer)
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.Sandbox{}).
		WithObjects(sandbox).
		Build()
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}}); err != nil {
		t.Fatal(err)
	}
	var updated computev1.Sandbox
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: sandbox.Name, Namespace: sandbox.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.Phase != computev1.SandboxPhaseFailed {
		t.Fatalf("expected failed phase, got %s", updated.Status.Phase)
	}
}

func testScheme(t *testing.T) *runtime.Scheme {
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
	return scheme
}
