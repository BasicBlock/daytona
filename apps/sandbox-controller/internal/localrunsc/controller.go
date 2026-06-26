package localrunsc

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SnapshotReconciler struct {
	client.Client
	Runtime  *Runtime
	NodeName string
}

func (r *SnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var snapshot computev1.LocalRunscSnapshot
	if err := r.Get(ctx, req.NamespacedName, &snapshot); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !snapshot.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &snapshot)
	}
	if snapshot.Spec.NodeName != r.NodeName {
		return ctrl.Result{}, nil
	}
	if !controllerutil.ContainsFinalizer(&snapshot, computev1.LocalRunscSnapshotFinalizer) {
		controllerutil.AddFinalizer(&snapshot, computev1.LocalRunscSnapshotFinalizer)
		return ctrl.Result{}, r.Update(ctx, &snapshot)
	}

	switch snapshot.Status.Phase {
	case computev1.LocalRunscSnapshotPhaseReady, computev1.LocalRunscSnapshotPhaseFailed:
		return ctrl.Result{}, nil
	case "", computev1.LocalRunscSnapshotPhasePending:
		phase := computev1.LocalRunscSnapshotPhaseRunning
		return ctrl.Result{Requeue: true}, r.updateStatus(ctx, &snapshot, localStatusPatch{
			Phase:    &phase,
			NodeName: &r.NodeName,
			Error:    ptr(""),
			Condition: condition(
				"Ready",
				metav1.ConditionFalse,
				"CheckpointRunning",
				"Local runsc checkpoint is running",
				snapshot.Generation,
			),
		})
	}

	sandboxID, err := r.sourceRuntimeContainerID(ctx, &snapshot)
	if err != nil {
		phase := computev1.LocalRunscSnapshotPhaseRunning
		message := err.Error()
		return ctrl.Result{RequeueAfter: 5 * time.Second}, r.updateStatus(ctx, &snapshot, localStatusPatch{
			Phase:    &phase,
			NodeName: &r.NodeName,
			Error:    &message,
			Condition: condition(
				"RuntimeContainerReady",
				metav1.ConditionFalse,
				"RuntimeContainerNotReady",
				message,
				snapshot.Generation,
			),
		})
	}

	result, err := r.Runtime.Checkpoint(ctx, CheckpointRequest{
		Namespace: snapshot.Namespace,
		Name:      snapshot.Name,
		SandboxID: sandboxID,
		Storage:   snapshot.Spec.Storage,
	})
	if err != nil {
		phase := computev1.LocalRunscSnapshotPhaseFailed
		message := err.Error()
		return ctrl.Result{}, r.updateStatus(ctx, &snapshot, localStatusPatch{
			Phase:    &phase,
			NodeName: &r.NodeName,
			Error:    &message,
			Condition: condition(
				"Ready",
				metav1.ConditionFalse,
				"CheckpointFailed",
				message,
				snapshot.Generation,
			),
		})
	}

	phase := computev1.LocalRunscSnapshotPhaseReady
	return ctrl.Result{}, r.updateStatus(ctx, &snapshot, localStatusPatch{
		Phase:      &phase,
		StorageRef: &result.StorageRef,
		NodeName:   &r.NodeName,
		Error:      ptr(""),
		Condition: condition(
			"Ready",
			metav1.ConditionTrue,
			"CheckpointReady",
			"Local runsc checkpoint completed",
			snapshot.Generation,
		),
	})
}

func (r *SnapshotReconciler) reconcileDelete(ctx context.Context, snapshot *computev1.LocalRunscSnapshot) (ctrl.Result, error) {
	if snapshot.Spec.NodeName != r.NodeName || !controllerutil.ContainsFinalizer(snapshot, computev1.LocalRunscSnapshotFinalizer) {
		return ctrl.Result{}, nil
	}
	err := r.Runtime.Cleanup(ctx, CleanupRequest{
		Namespace:  snapshot.Namespace,
		Name:       snapshot.Name,
		StorageRef: snapshot.Status.StorageRef,
		Storage:    snapshot.Spec.Storage,
	})
	if err != nil {
		message := err.Error()
		return ctrl.Result{RequeueAfter: 10 * time.Second}, r.updateStatus(ctx, snapshot, localStatusPatch{
			NodeName: &r.NodeName,
			Error:    &message,
			Condition: condition(
				"Cleanup",
				metav1.ConditionFalse,
				"CleanupFailed",
				message,
				snapshot.Generation,
			),
		})
	}
	controllerutil.RemoveFinalizer(snapshot, computev1.LocalRunscSnapshotFinalizer)
	return ctrl.Result{}, r.Update(ctx, snapshot)
}

func (r *SnapshotReconciler) sourceRuntimeContainerID(ctx context.Context, snapshot *computev1.LocalRunscSnapshot) (string, error) {
	containerName := snapshot.Spec.SourceContainerName
	if containerName == "" {
		containerName = "workload"
	}
	var pod corev1.Pod
	key := types.NamespacedName{Name: snapshot.Spec.SourcePodName, Namespace: snapshot.Namespace}
	if err := r.Get(ctx, key, &pod); err != nil {
		return "", err
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != containerName {
			continue
		}
		if status.ContainerID == "" {
			return "", fmt.Errorf("container %s has no runtime container ID yet", containerName)
		}
		return stripRuntimePrefix(status.ContainerID), nil
	}
	return "", fmt.Errorf("container %s not found in Pod %s", containerName, snapshot.Spec.SourcePodName)
}

func stripRuntimePrefix(containerID string) string {
	if index := strings.LastIndex(containerID, "://"); index >= 0 {
		return containerID[index+3:]
	}
	return containerID
}

func (r *SnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.LocalRunscSnapshot{}).
		Complete(r)
}

type localStatusPatch struct {
	Phase      *computev1.LocalRunscSnapshotPhase
	StorageRef *string
	NodeName   *string
	Error      *string
	Condition  *metav1.Condition
}

func (r *SnapshotReconciler) updateStatus(ctx context.Context, snapshot *computev1.LocalRunscSnapshot, patch localStatusPatch) error {
	next := snapshot.DeepCopyObject().(*computev1.LocalRunscSnapshot)
	next.Status.ObservedGeneration = snapshot.Generation
	if patch.Phase != nil {
		next.Status.Phase = *patch.Phase
	}
	if patch.StorageRef != nil {
		next.Status.StorageRef = *patch.StorageRef
	}
	if patch.NodeName != nil {
		next.Status.NodeName = *patch.NodeName
	}
	if patch.Error != nil {
		next.Status.Error = *patch.Error
	}
	if patch.Condition != nil {
		meta.SetStatusCondition(&next.Status.Conditions, *patch.Condition)
	}
	if reflect.DeepEqual(snapshot.Status, next.Status) {
		return nil
	}
	err := r.Status().Update(ctx, next)
	if apierrors.IsConflict(err) {
		return nil
	}
	return err
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

func ptr[T any](value T) *T {
	return &value
}
