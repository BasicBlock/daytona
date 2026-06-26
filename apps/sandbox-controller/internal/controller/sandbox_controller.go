package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/observability"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SandboxReconciler struct {
	client.Client
	Scheme              *runtime.Scheme
	DefaultToolboxImage string
	Recorder            record.EventRecorder
	StaleDeleteTimeout  time.Duration
	Now                 func() time.Time
}

func (r *SandboxReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	started := time.Now()
	defer func() { observability.ObserveReconcile("sandbox", started, err) }()

	var sandbox computev1.Sandbox
	if err := r.Get(ctx, req.NamespacedName, &sandbox); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !sandbox.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &sandbox)
	}

	if !controllerutil.ContainsFinalizer(&sandbox, computev1.SandboxFinalizer) {
		controllerutil.AddFinalizer(&sandbox, computev1.SandboxFinalizer)
		return ctrl.Result{}, r.Update(ctx, &sandbox)
	}

	if render.DesiredState(&sandbox) == computev1.SandboxDesiredStateStopped {
		return r.reconcileStopped(ctx, &sandbox)
	}

	if err := r.reconcileService(ctx, &sandbox); err != nil {
		return ctrl.Result{}, err
	}
	if err := r.reconcileNetworkPolicy(ctx, &sandbox); err != nil {
		return ctrl.Result{}, err
	}

	return r.reconcileRunning(ctx, &sandbox)
}

func (r *SandboxReconciler) reconcileDelete(ctx context.Context, sandbox *computev1.Sandbox) (ctrl.Result, error) {
	phase := computev1.SandboxPhaseDeleting
	_ = r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{Phase: &phase})
	stale := r.isStaleDelete(sandbox)
	for _, obj := range []client.Object{
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: render.ServiceName(sandbox), Namespace: sandbox.Namespace}},
		&networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: render.NetworkPolicyName(sandbox), Namespace: sandbox.Namespace}},
	} {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) {
			if !stale {
				return ctrl.Result{}, err
			}
		}
	}

	var latest computev1.Sandbox
	if err := r.Get(ctx, client.ObjectKeyFromObject(sandbox), &latest); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !controllerutil.ContainsFinalizer(&latest, computev1.SandboxFinalizer) {
		return ctrl.Result{}, nil
	}
	controllerutil.RemoveFinalizer(&latest, computev1.SandboxFinalizer)
	return ctrl.Result{}, r.Update(ctx, &latest)
}

func (r *SandboxReconciler) isStaleDelete(sandbox *computev1.Sandbox) bool {
	if r.StaleDeleteTimeout <= 0 || sandbox.DeletionTimestamp == nil {
		return false
	}
	return r.now().Sub(sandbox.DeletionTimestamp.Time) > r.StaleDeleteTimeout
}

func (r *SandboxReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *SandboxReconciler) reconcileStopped(ctx context.Context, sandbox *computev1.Sandbox) (ctrl.Result, error) {
	var pod corev1.Pod
	err := r.Get(ctx, types.NamespacedName{Name: render.PodName(sandbox), Namespace: sandbox.Namespace}, &pod)
	if err == nil {
		if sandbox.Spec.StopPolicy.SnapshotBeforeStop {
			ready, patch, err := r.ensureStopSnapshot(ctx, sandbox)
			if err != nil {
				return ctrl.Result{}, err
			}
			if !ready {
				return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, patch)
			}
		}
		if err := r.Delete(ctx, &pod); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	} else if !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	serviceName := render.ServiceName(sandbox)
	phase := computev1.SandboxPhaseStopped
	return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{
		Phase:       &phase,
		PodName:     ptr(""),
		ServiceName: &serviceName,
	})
}

func (r *SandboxReconciler) ensureStopSnapshot(ctx context.Context, sandbox *computev1.Sandbox) (bool, SandboxStatusPatch, error) {
	if sandbox.Spec.StopPolicy.SnapshotName == "" {
		phase := computev1.SandboxPhaseFailed
		return false, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"StopSnapshotReady",
				metav1.ConditionFalse,
				"MissingStopSnapshotName",
				"spec.stopPolicy.snapshotName is required when snapshotBeforeStop is true",
				sandbox.Generation,
			),
		}, nil
	}

	var snapshot computev1.SandboxSnapshot
	key := types.NamespacedName{Name: sandbox.Spec.StopPolicy.SnapshotName, Namespace: sandbox.Namespace}
	if err := r.Get(ctx, key, &snapshot); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, SandboxStatusPatch{}, err
		}

		provider := sandbox.Spec.StopPolicy.Provider
		if provider == "" {
			provider = computev1.SnapshotProviderGKEPodSnapshot
		}
		snapshot = computev1.SandboxSnapshot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sandbox.Spec.StopPolicy.SnapshotName,
				Namespace: sandbox.Namespace,
			},
			Spec: computev1.SandboxSnapshotSpec{
				Provider: provider,
				Source: computev1.SandboxSnapshotSourceRef{
					SandboxName: sandbox.Name,
				},
				GKE:   sandbox.Spec.StopPolicy.GKE,
				Local: sandbox.Spec.StopPolicy.Local,
			},
		}
		if err := controllerutil.SetControllerReference(sandbox, &snapshot, r.Scheme); err != nil {
			return false, SandboxStatusPatch{}, err
		}
		if err := r.Create(ctx, &snapshot); err != nil {
			return false, SandboxStatusPatch{}, err
		}
	}

	if snapshot.Status.Phase == computev1.SandboxSnapshotPhaseReady {
		return true, SandboxStatusPatch{}, nil
	}

	phase := computev1.SandboxPhaseStopping
	return false, SandboxStatusPatch{
		Phase:   &phase,
		PodName: ptr(render.PodName(sandbox)),
		Condition: condition(
			"StopSnapshotReady",
			metav1.ConditionFalse,
			"WaitingForStopSnapshot",
			"Waiting for stop snapshot to become ready before deleting the sandbox Pod",
			sandbox.Generation,
		),
	}, nil
}

func (r *SandboxReconciler) reconcileService(ctx context.Context, sandbox *computev1.Sandbox) error {
	desired := render.Service(sandbox)
	if err := controllerutil.SetControllerReference(sandbox, desired, r.Scheme); err != nil {
		return err
	}

	var existing corev1.Service
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		return err
	}

	changed := false
	if !reflect.DeepEqual(existing.Labels, desired.Labels) {
		existing.Labels = desired.Labels
		changed = true
	}
	if !reflect.DeepEqual(existing.Spec.Selector, desired.Spec.Selector) {
		existing.Spec.Selector = desired.Spec.Selector
		changed = true
	}
	if !reflect.DeepEqual(existing.Spec.Ports, desired.Spec.Ports) {
		existing.Spec.Ports = desired.Spec.Ports
		changed = true
	}
	if changed {
		return r.Update(ctx, &existing)
	}
	return nil
}

func (r *SandboxReconciler) reconcileNetworkPolicy(ctx context.Context, sandbox *computev1.Sandbox) error {
	desired := render.NetworkPolicy(sandbox)
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}

	if !sandbox.Spec.NetworkPolicy.Enabled {
		var existing networkingv1.NetworkPolicy
		if err := r.Get(ctx, key, &existing); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		return client.IgnoreNotFound(r.Delete(ctx, &existing))
	}

	if err := controllerutil.SetControllerReference(sandbox, desired, r.Scheme); err != nil {
		return err
	}

	var existing networkingv1.NetworkPolicy
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		return err
	}

	changed := false
	if !reflect.DeepEqual(existing.Labels, desired.Labels) {
		existing.Labels = desired.Labels
		changed = true
	}
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		changed = true
	}
	if changed {
		return r.Update(ctx, &existing)
	}
	return nil
}

func (r *SandboxReconciler) reconcileRunning(ctx context.Context, sandbox *computev1.Sandbox) (ctrl.Result, error) {
	effectiveSandbox := sandbox.DeepCopyObject().(*computev1.Sandbox)
	specHash, err := render.CompatibilityHash(effectiveSandbox)
	if err != nil {
		phase := computev1.SandboxPhaseFailed
		return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"Renderable",
				metav1.ConditionFalse,
				"InvalidSandboxSpec",
				err.Error(),
				sandbox.Generation,
			),
		})
	}

	if ok, patch, err := r.resolveRestore(ctx, sandbox, effectiveSandbox, specHash); err != nil {
		return ctrl.Result{}, err
	} else if !ok {
		return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, patch)
	}

	desired, specHash, err := render.Pod(effectiveSandbox, r.DefaultToolboxImage)
	if err != nil {
		phase := computev1.SandboxPhaseFailed
		return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"Renderable",
				metav1.ConditionFalse,
				"InvalidSandboxSpec",
				err.Error(),
				sandbox.Generation,
			),
		})
	}
	if err := controllerutil.SetControllerReference(sandbox, desired, r.Scheme); err != nil {
		return ctrl.Result{}, err
	}

	var existing corev1.Pod
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}
	if err := r.Get(ctx, key, &existing); err != nil {
		if !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, desired); err != nil {
			return ctrl.Result{}, err
		}
		phase := computev1.SandboxPhaseStarting
		if sandbox.Spec.Restore != nil {
			phase = computev1.SandboxPhaseRestoring
		}
		return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{
			Phase:       &phase,
			PodName:     &desired.Name,
			ServiceName: ptr(render.ServiceName(sandbox)),
			SpecHash:    &specHash,
		})
	}

	if existing.Annotations[computev1.AnnotationSpecHash] != specHash {
		if err := r.Delete(ctx, &existing); err != nil && !apierrors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		phase := computev1.SandboxPhaseStarting
		return ctrl.Result{Requeue: true}, r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{
			Phase:    &phase,
			PodName:  &existing.Name,
			SpecHash: &specHash,
		})
	}

	phase := phaseFromPod(&existing, sandbox)
	return ctrl.Result{}, r.updateSandboxStatus(ctx, sandbox, SandboxStatusPatch{
		Phase:            &phase,
		PodName:          &existing.Name,
		ServiceName:      ptr(render.ServiceName(sandbox)),
		SpecHash:         &specHash,
		RestoredSnapshot: restoredSnapshotName(sandbox),
		Condition: condition(
			"Ready",
			conditionStatusFromPhase(phase),
			string(phase),
			"Sandbox workload reconciled",
			sandbox.Generation,
		),
	})
}

func (r *SandboxReconciler) resolveRestore(ctx context.Context, original *computev1.Sandbox, effective *computev1.Sandbox, specHash string) (bool, SandboxStatusPatch, error) {
	if effective.Spec.Restore == nil {
		return true, SandboxStatusPatch{}, nil
	}
	if effective.Spec.Restore.Name == "" {
		phase := computev1.SandboxPhaseFailed
		return false, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"RestoreReady",
				metav1.ConditionFalse,
				"MissingRestoreName",
				"spec.restore.name is required",
				original.Generation,
			),
		}, nil
	}
	if render.HasPersistentVolumeClaim(effective) {
		phase := computev1.SandboxPhaseFailed
		return false, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"RestoreReady",
				metav1.ConditionFalse,
				"PersistentVolumeClaimUnsupported",
				"PVC-backed sandbox volumes are unsupported for v1 restore",
				original.Generation,
			),
		}, nil
	}

	var snapshot computev1.SandboxSnapshot
	key := types.NamespacedName{Name: effective.Spec.Restore.Name, Namespace: effective.Namespace}
	if err := r.Get(ctx, key, &snapshot); err != nil {
		if apierrors.IsNotFound(err) {
			phase := computev1.SandboxPhaseFailed
			return false, SandboxStatusPatch{
				Phase: &phase,
				Condition: condition(
					"RestoreReady",
					metav1.ConditionFalse,
					"SnapshotNotFound",
					err.Error(),
					original.Generation,
				),
			}, nil
		}
		return false, SandboxStatusPatch{}, err
	}

	if snapshot.Status.Phase != computev1.SandboxSnapshotPhaseReady || snapshot.Status.ProviderObjectName == "" {
		phase := computev1.SandboxPhaseRestoring
		return false, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"RestoreReady",
				metav1.ConditionFalse,
				"SnapshotNotReady",
				"Referenced SandboxSnapshot is not ready for restore",
				original.Generation,
			),
		}, nil
	}

	if snapshot.Status.TemplateName != "" {
		if effective.Spec.TemplateName != snapshot.Status.TemplateName {
			phase := computev1.SandboxPhaseFailed
			return false, SandboxStatusPatch{
				Phase: &phase,
				Condition: condition(
					"RestoreReady",
					metav1.ConditionFalse,
					"TemplateMismatch",
					fmt.Sprintf("restore requires SandboxTemplate %s", snapshot.Status.TemplateName),
					original.Generation,
				),
			}, nil
		}
		templateHash, err := r.templateCompatibilityHash(ctx, effective.Namespace, effective.Spec.TemplateName)
		if err != nil {
			return false, SandboxStatusPatch{}, err
		}
		if templateHash != specHash {
			phase := computev1.SandboxPhaseFailed
			return false, SandboxStatusPatch{
				Phase: &phase,
				Condition: condition(
					"RestoreReady",
					metav1.ConditionFalse,
					"TemplateSpecMismatch",
					fmt.Sprintf("sandbox spec hash %s does not match template compatibility hash %s", specHash, templateHash),
					original.Generation,
				),
			}, nil
		}
		specHash = templateHash
	}

	if snapshot.Status.CompatibilityHash != "" && snapshot.Status.CompatibilityHash != specHash {
		phase := computev1.SandboxPhaseFailed
		return false, SandboxStatusPatch{
			Phase: &phase,
			Condition: condition(
				"RestoreReady",
				metav1.ConditionFalse,
				"IncompatibleSnapshot",
				fmt.Sprintf("sandbox spec hash %s does not match snapshot compatibility hash %s", specHash, snapshot.Status.CompatibilityHash),
				original.Generation,
			),
		}, nil
	}

	provider := snapshot.Spec.Provider
	if provider == "" {
		provider = computev1.SnapshotProviderGKEPodSnapshot
	}
	effective.Spec.Restore.ProviderObjectName = snapshot.Status.ProviderObjectName
	effective.Spec.Restore.Provider = provider
	effective.Spec.Restore.StorageRef = snapshot.Status.StorageRef
	return true, SandboxStatusPatch{}, nil
}

func (r *SandboxReconciler) templateCompatibilityHash(ctx context.Context, namespace string, name string) (string, error) {
	var template computev1.SandboxTemplate
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, &template); err != nil {
		return "", err
	}
	if template.Status.CompatibilityHash != "" {
		return template.Status.CompatibilityHash, nil
	}
	return render.CompatibilityHash(&computev1.Sandbox{
		ObjectMeta: metav1.ObjectMeta{Name: template.Name, Namespace: template.Namespace},
		Spec:       template.Spec.Template,
	})
}

type SandboxStatusPatch struct {
	Phase            *computev1.SandboxPhase
	PodName          *string
	ServiceName      *string
	SpecHash         *string
	RestoredSnapshot *string
	Condition        *metav1.Condition
}

func (r *SandboxReconciler) updateSandboxStatus(ctx context.Context, sandbox *computev1.Sandbox, patch SandboxStatusPatch) error {
	next := sandbox.DeepCopyObject().(*computev1.Sandbox)
	next.Status.ObservedGeneration = sandbox.Generation
	if patch.Phase != nil {
		next.Status.Phase = *patch.Phase
	}
	if patch.PodName != nil {
		next.Status.PodName = *patch.PodName
	}
	if patch.ServiceName != nil {
		next.Status.ServiceName = *patch.ServiceName
	}
	if patch.SpecHash != nil {
		next.Status.SpecHash = *patch.SpecHash
	}
	if patch.RestoredSnapshot != nil {
		next.Status.RestoredSnapshot = *patch.RestoredSnapshot
	}
	if patch.Condition != nil {
		meta.SetStatusCondition(&next.Status.Conditions, *patch.Condition)
	}
	if reflect.DeepEqual(sandbox.Status, next.Status) {
		return nil
	}
	if err := r.Status().Update(ctx, next); err != nil {
		return err
	}
	if patch.Phase != nil {
		observability.SandboxPhase.WithLabelValues(sandbox.Namespace, sandbox.Name, string(*patch.Phase)).Set(1)
		if r.Recorder != nil && sandbox.Status.Phase != *patch.Phase {
			r.Recorder.Eventf(sandbox, corev1.EventTypeNormal, "SandboxPhaseChanged", "Sandbox phase changed from %s to %s", sandbox.Status.Phase, *patch.Phase)
		}
	}
	return nil
}

func phaseFromPod(pod *corev1.Pod, sandbox *computev1.Sandbox) computev1.SandboxPhase {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		for _, cond := range pod.Status.Conditions {
			if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
				return computev1.SandboxPhaseRunning
			}
		}
		return computev1.SandboxPhaseStarting
	case corev1.PodFailed:
		return computev1.SandboxPhaseFailed
	case corev1.PodSucceeded:
		return computev1.SandboxPhaseStopped
	case corev1.PodPending:
		if sandbox.Spec.Restore != nil {
			return computev1.SandboxPhaseRestoring
		}
		return computev1.SandboxPhaseStarting
	default:
		return computev1.SandboxPhaseUnknown
	}
}

func conditionStatusFromPhase(phase computev1.SandboxPhase) metav1.ConditionStatus {
	if phase == computev1.SandboxPhaseRunning {
		return metav1.ConditionTrue
	}
	return metav1.ConditionFalse
}

func condition(conditionType string, status metav1.ConditionStatus, reason string, message string, generation int64) *metav1.Condition {
	return &metav1.Condition{
		Type:               conditionType,
		Status:             status,
		ObservedGeneration: generation,
		Reason:             reason,
		Message:            message,
	}
}

func restoredSnapshotName(sandbox *computev1.Sandbox) *string {
	if sandbox.Spec.Restore == nil {
		return nil
	}
	return &sandbox.Spec.Restore.Name
}

func ptr[T any](value T) *T {
	return &value
}

func (r *SandboxReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.Sandbox{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Complete(r)
}
