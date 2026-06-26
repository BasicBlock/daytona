package controller

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	nodev1 "k8s.io/api/node/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestEnvtestSandboxControllerBehavior(t *testing.T) {
	if os.Getenv("DAYTONA_ENVTEST") != "1" {
		t.Skip("set DAYTONA_ENVTEST=1 to run controller envtest coverage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := nodev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := computev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	testEnv := &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "..", "config", "crd")},
	}
	cfg, err := testEnv.Start()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = testEnv.Stop()
	})

	k8sClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatal(err)
	}
	reconciler := &SandboxReconciler{Client: k8sClient, Scheme: scheme}
	if err := k8sClient.Create(ctx, &nodev1.RuntimeClass{ObjectMeta: metav1.ObjectMeta{Name: render.DefaultRuntimeClassName}, Handler: render.DefaultRuntimeClassName}); err != nil {
		t.Fatal(err)
	}

	invalid := &computev1.Sandbox{ObjectMeta: metav1.ObjectMeta{Name: "invalid", Namespace: "default"}}
	if err := k8sClient.Create(ctx, invalid); !apierrors.IsInvalid(err) {
		t.Fatalf("expected CRD validation to reject missing image, got %v", err)
	}

	localSource := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "local-source", Namespace: "default"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	if err := k8sClient.Create(ctx, localSource); err != nil {
		t.Fatal(err)
	}
	localPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: render.PodName(localSource), Namespace: localSource.Namespace},
		Spec: corev1.PodSpec{
			NodeName: "node-a",
			Containers: []corev1.Container{{
				Name:  render.WorkloadContainerName,
				Image: "ubuntu:24.04",
			}},
		},
	}
	if err := k8sClient.Create(ctx, localPod); err != nil {
		t.Fatal(err)
	}
	localSnapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "local-warm", Namespace: "default"},
		Spec: computev1.SandboxSnapshotSpec{
			Provider: computev1.SnapshotProviderLocalRunsc,
			Source:   computev1.SandboxSnapshotSourceRef{SandboxName: localSource.Name},
			Local: computev1.LocalRunscProviderSpec{
				Storage: computev1.LocalRunscStorageSpec{Mode: "filesystem", Path: "/snapshots"},
			},
		},
	}
	if err := k8sClient.Create(ctx, localSnapshot); err != nil {
		t.Fatal(err)
	}
	snapshotReconciler := &SandboxSnapshotReconciler{Client: k8sClient, Scheme: scheme}
	snapshotReq := ctrl.Request{NamespacedName: types.NamespacedName{Name: localSnapshot.Name, Namespace: localSnapshot.Namespace}}
	if _, err := snapshotReconciler.Reconcile(ctx, snapshotReq); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotReconciler.Reconcile(ctx, snapshotReq); err != nil {
		t.Fatal(err)
	}
	var localRequest computev1.LocalRunscSnapshot
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: "lrs-local-warm", Namespace: "default"}, &localRequest); err != nil {
		t.Fatal(err)
	}
	if localRequest.OwnerReferences[0].Name != localSnapshot.Name {
		t.Fatalf("expected LocalRunscSnapshot owner %s, got %#v", localSnapshot.Name, localRequest.OwnerReferences)
	}
	localRequest.Status.Phase = computev1.LocalRunscSnapshotPhaseReady
	localRequest.Status.StorageRef = "/snapshots/local-warm"
	if err := k8sClient.Status().Update(ctx, &localRequest); err != nil {
		t.Fatal(err)
	}
	if _, err := snapshotReconciler.Reconcile(ctx, snapshotReq); err != nil {
		t.Fatal(err)
	}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: localSnapshot.Name, Namespace: localSnapshot.Namespace}, localSnapshot); err != nil {
		t.Fatal(err)
	}
	if localSnapshot.Status.Phase != computev1.SandboxSnapshotPhaseReady || localSnapshot.Status.StorageRef != "/snapshots/local-warm" {
		t.Fatalf("expected local snapshot ready status, got %#v", localSnapshot.Status)
	}

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec:       computev1.SandboxSpec{Image: "ubuntu:24.04"},
	}
	if err := k8sClient.Create(ctx, sandbox); err != nil {
		t.Fatal(err)
	}
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
	if pod.OwnerReferences[0].Name != sandbox.Name {
		t.Fatalf("expected sandbox owner reference, got %#v", pod.OwnerReferences)
	}

	var service corev1.Service
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.ServiceName(sandbox), Namespace: sandbox.Namespace}, &service); err != nil {
		t.Fatal(err)
	}

	if err := k8sClient.Delete(ctx, sandbox); err != nil {
		t.Fatal(err)
	}
	if _, err := reconciler.Reconcile(ctx, req); err != nil && !apierrors.IsConflict(err) {
		t.Fatal(err)
	}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &pod); !apierrors.IsNotFound(err) {
		t.Fatalf("expected sandbox Pod to be deleted, got %v", err)
	}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: render.ServiceName(sandbox), Namespace: sandbox.Namespace}, &service); !apierrors.IsNotFound(err) {
		t.Fatalf("expected sandbox Service to be deleted, got %v", err)
	}
}
