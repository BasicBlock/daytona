package controller

import (
	"context"
	"reflect"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/render"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type SandboxTemplateReconciler struct {
	client.Client
}

func (r *SandboxTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var template computev1.SandboxTemplate
	if err := r.Get(ctx, req.NamespacedName, &template); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	sandbox := &computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{
			Name:      template.Name,
			Namespace: template.Namespace,
		},
		Spec: template.Spec.Template,
	}
	hash, err := render.CompatibilityHash(sandbox)
	if err != nil {
		return ctrl.Result{}, err
	}

	next := template.DeepCopyObject().(*computev1.SandboxTemplate)
	next.Status.ObservedGeneration = template.Generation
	next.Status.CompatibilityHash = hash
	if reflect.DeepEqual(template.Status, next.Status) {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{}, r.Status().Update(ctx, next)
}

func (r *SandboxTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.SandboxTemplate{}).
		Complete(r)
}
