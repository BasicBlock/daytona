package controller

import (
	"context"
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestSandboxTemplateReconcilerWritesCompatibilityHash(t *testing.T) {
	ctx := context.Background()
	scheme := testScheme(t)
	template := &computev1.SandboxTemplate{
		ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "default"},
		Spec: computev1.SandboxTemplateSpec{
			Template: computev1.SandboxSpec{Image: "ubuntu:24.04"},
		},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&computev1.SandboxTemplate{}).
		WithObjects(template).
		Build()

	reconciler := &SandboxTemplateReconciler{Client: k8sClient}
	_, err := reconciler.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: template.Name, Namespace: template.Namespace},
	})
	if err != nil {
		t.Fatal(err)
	}

	var updated computev1.SandboxTemplate
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: template.Name, Namespace: template.Namespace}, &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Status.CompatibilityHash == "" {
		t.Fatalf("expected compatibility hash, got %#v", updated.Status)
	}
}
