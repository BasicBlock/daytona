package controller

import (
	"context"
	"fmt"
	"reflect"
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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type LocalPodSnapshotShimReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	DefaultStorage computev1.LocalRunscStorageSpec
}

func (r *LocalPodSnapshotShimReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	trigger := gkeManualTrigger()
	if err := r.Get(ctx, req.NamespacedName, trigger); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !trigger.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	targetPodName, _, _ := unstructured.NestedString(trigger.Object, "spec", "targetPod")
	if targetPodName == "" {
		return ctrl.Result{}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "MissingTargetPod", "spec.targetPod is required")
	}

	var pod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Name: targetPodName, Namespace: trigger.GetNamespace()}, &pod); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "TargetPodNotFound", err.Error())
		}
		return ctrl.Result{}, err
	}
	if pod.Spec.NodeName == "" {
		return ctrl.Result{RequeueAfter: 2 * time.Second}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "TargetPodNotScheduled", "target Pod is not scheduled")
	}

	sandboxName := pod.Labels[computev1.LabelSandboxName]
	if sandboxName == "" {
		return ctrl.Result{}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "MissingSandboxLabel", fmt.Sprintf("target Pod %s is missing %s", pod.Name, computev1.LabelSandboxName))
	}

	storage, policyName, err := r.localStorageForTrigger(ctx, trigger)
	if err != nil {
		return ctrl.Result{}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "StorageConfigNotReady", err.Error())
	}

	request, err := r.ensureLocalRunscRequest(ctx, trigger, &pod, sandboxName, storage)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch request.Status.Phase {
	case computev1.LocalRunscSnapshotPhaseReady:
		podSnapshotName := gke.ObjectName("ps", trigger.GetName())
		podSnapshot := r.localPodSnapshot(trigger, podSnapshotName, policyName, request.Status.StorageRef)
		if err := r.createOrPatchUnstructured(ctx, podSnapshot); err != nil {
			return ctrl.Result{}, err
		}
		if err := unstructured.SetNestedField(trigger.Object, podSnapshotName, "status", "snapshotCreated"); err != nil {
			return ctrl.Result{}, err
		}
		if err := setUnstructuredCondition(trigger, metav1.ConditionTrue, "Ready", "Local PodSnapshot shim completed runsc checkpoint"); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, r.Status().Update(ctx, trigger)
	case computev1.LocalRunscSnapshotPhaseFailed:
		message := request.Status.Error
		if message == "" {
			message = "LocalRunscSnapshot failed"
		}
		return ctrl.Result{}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "LocalRunscSnapshotFailed", message)
	default:
		return ctrl.Result{RequeueAfter: 2 * time.Second}, r.patchTriggerCondition(ctx, trigger, metav1.ConditionFalse, "WaitingForLocalRunscSnapshot", "waiting for local runsc snapshot request")
	}
}

func (r *LocalPodSnapshotShimReconciler) localStorageForTrigger(ctx context.Context, trigger *unstructured.Unstructured) (computev1.LocalRunscStorageSpec, string, error) {
	policyName := trigger.GetLabels()[gke.LabelSnapshotName]
	if policyName != "" {
		policyName = gke.ObjectName("psp", policyName)
	}
	if policyName == "" {
		var policies unstructured.UnstructuredList
		policies.SetAPIVersion(gke.APIVersion)
		policies.SetKind(gke.PolicyKind + "List")
		if err := r.List(ctx, &policies, client.InNamespace(trigger.GetNamespace())); err != nil {
			return computev1.LocalRunscStorageSpec{}, "", err
		}
		for i := range policies.Items {
			if policies.Items[i].GetLabels()[gke.LabelSnapshotName] == trigger.GetLabels()[gke.LabelSnapshotName] {
				policyName = policies.Items[i].GetName()
				break
			}
		}
	}
	if policyName == "" {
		return computev1.LocalRunscStorageSpec{}, "", fmt.Errorf("matching PodSnapshotPolicy was not found")
	}

	policy := gkePolicy()
	if err := r.Get(ctx, types.NamespacedName{Name: policyName, Namespace: trigger.GetNamespace()}, policy); err != nil {
		return computev1.LocalRunscStorageSpec{}, "", err
	}
	storageConfigName, _, _ := unstructured.NestedString(policy.Object, "spec", "storageConfigName")
	storage := r.DefaultStorage
	if storageConfigName == "" {
		return storage, policyName, nil
	}

	storageConfig := gkeStorageConfig()
	if err := r.Get(ctx, types.NamespacedName{Name: storageConfigName}, storageConfig); err != nil {
		return computev1.LocalRunscStorageSpec{}, "", err
	}
	bucket, _, _ := unstructured.NestedString(storageConfig.Object, "spec", "snapshotStorageConfig", "gcs", "bucket")
	path, _, _ := unstructured.NestedString(storageConfig.Object, "spec", "snapshotStorageConfig", "gcs", "path")
	if bucket != "" {
		storage.Mode = "s3"
		storage.Bucket = bucket
	}
	if path != "" {
		storage.Prefix = path
	}
	return storage, policyName, nil
}

func (r *LocalPodSnapshotShimReconciler) ensureLocalRunscRequest(ctx context.Context, trigger *unstructured.Unstructured, pod *corev1.Pod, sandboxName string, storage computev1.LocalRunscStorageSpec) (*computev1.LocalRunscSnapshot, error) {
	desired := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gke.ObjectName("lrs", trigger.GetName()),
			Namespace: trigger.GetNamespace(),
			Labels: map[string]string{
				computev1.LabelManagedBy:   computev1.ManagedByValue,
				computev1.LabelSandboxName: sandboxName,
				gke.LabelSnapshotName:      trigger.GetLabels()[gke.LabelSnapshotName],
			},
			OwnerReferences: []metav1.OwnerReference{ownerReference(trigger)},
		},
		Spec: computev1.LocalRunscSnapshotSpec{
			SandboxName:         sandboxName,
			SourcePodName:       pod.Name,
			SourceContainerName: render.WorkloadContainerName,
			NodeName:            pod.Spec.NodeName,
			Storage:             storage,
		},
	}

	var existing computev1.LocalRunscSnapshot
	key := types.NamespacedName{Name: desired.Name, Namespace: desired.Namespace}
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, desired); err != nil {
				return nil, err
			}
			return desired, nil
		}
		return nil, err
	}
	changed := false
	if !reflect.DeepEqual(existing.Labels, desired.Labels) {
		existing.Labels = desired.Labels
		changed = true
	}
	if !reflect.DeepEqual(existing.OwnerReferences, desired.OwnerReferences) {
		existing.OwnerReferences = desired.OwnerReferences
		changed = true
	}
	if !reflect.DeepEqual(existing.Spec, desired.Spec) {
		existing.Spec = desired.Spec
		changed = true
	}
	if changed {
		status := existing.Status
		if err := r.Update(ctx, &existing); err != nil {
			return nil, err
		}
		existing.Status = status
	}
	return &existing, nil
}

func (r *LocalPodSnapshotShimReconciler) localPodSnapshot(trigger *unstructured.Unstructured, name string, policyName string, storageRef string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"spec": map[string]any{
			"policyName": policyName,
		},
		"status": map[string]any{
			"artifactStorageRef": storageRef,
			"conditions": []any{map[string]any{
				"type":               "Ready",
				"status":             "True",
				"reason":             "Ready",
				"message":            "Local runsc checkpoint is ready",
				"observedGeneration": int64(1),
				"lastTransitionTime": metav1.Now().Format(time.RFC3339),
			}},
		},
	}}
	obj.SetAPIVersion(gke.APIVersion)
	obj.SetKind(gke.PodSnapshotKind)
	obj.SetName(name)
	obj.SetNamespace(trigger.GetNamespace())
	obj.SetLabels(map[string]string{
		computev1.LabelManagedBy: computev1.ManagedByValue,
		gke.LabelSnapshotName:    trigger.GetLabels()[gke.LabelSnapshotName],
	})
	obj.SetOwnerReferences([]metav1.OwnerReference{ownerReference(trigger)})
	return obj
}

func (r *LocalPodSnapshotShimReconciler) createOrPatchUnstructured(ctx context.Context, desired *unstructured.Unstructured) error {
	var existing unstructured.Unstructured
	existing.SetAPIVersion(desired.GetAPIVersion())
	existing.SetKind(desired.GetKind())
	desiredStatus, hasDesiredStatus := desired.Object["status"]

	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			create := desired.DeepCopy()
			delete(create.Object, "status")
			if err := r.Create(ctx, create); err != nil {
				return err
			}
			if !hasDesiredStatus {
				return nil
			}
			return r.patchUnstructuredStatus(ctx, desired.GetAPIVersion(), desired.GetKind(), key, desiredStatus)
		}
		return err
	}

	changed := false
	if !reflect.DeepEqual(existing.GetLabels(), desired.GetLabels()) {
		existing.SetLabels(desired.GetLabels())
		changed = true
	}
	if !reflect.DeepEqual(existing.GetOwnerReferences(), desired.GetOwnerReferences()) {
		existing.SetOwnerReferences(desired.GetOwnerReferences())
		changed = true
	}
	if !reflect.DeepEqual(existing.Object["spec"], desired.Object["spec"]) {
		existing.Object["spec"] = desired.Object["spec"]
		changed = true
	}
	if changed {
		if err := r.Update(ctx, &existing); err != nil {
			return err
		}
	}
	if hasDesiredStatus && !reflect.DeepEqual(existing.Object["status"], desiredStatus) {
		return r.patchUnstructuredStatus(ctx, desired.GetAPIVersion(), desired.GetKind(), key, desiredStatus)
	}
	return nil
}

func (r *LocalPodSnapshotShimReconciler) patchUnstructuredStatus(ctx context.Context, apiVersion string, kind string, key types.NamespacedName, status any) error {
	var current unstructured.Unstructured
	current.SetAPIVersion(apiVersion)
	current.SetKind(kind)
	if err := r.Get(ctx, key, &current); err != nil {
		return err
	}
	current.Object["status"] = status
	return r.Status().Update(ctx, &current)
}

func (r *LocalPodSnapshotShimReconciler) patchTriggerCondition(ctx context.Context, trigger *unstructured.Unstructured, status metav1.ConditionStatus, reason string, message string) error {
	if err := setUnstructuredCondition(trigger, status, reason, message); err != nil {
		return err
	}
	return r.Status().Update(ctx, trigger)
}

func setUnstructuredCondition(obj *unstructured.Unstructured, status metav1.ConditionStatus, reason string, message string) error {
	condition := map[string]any{
		"type":               "Ready",
		"status":             string(status),
		"reason":             reason,
		"message":            message,
		"observedGeneration": obj.GetGeneration(),
		"lastTransitionTime": metav1.Now().Format(time.RFC3339),
	}
	return unstructured.SetNestedSlice(obj.Object, []any{condition}, "status", "conditions")
}

func ownerReference(obj *unstructured.Unstructured) metav1.OwnerReference {
	controller := true
	blockOwnerDeletion := true
	return metav1.OwnerReference{
		APIVersion:         obj.GetAPIVersion(),
		Kind:               obj.GetKind(),
		Name:               obj.GetName(),
		UID:                obj.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}
}

func gkeManualTrigger() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(gke.APIVersion)
	obj.SetKind(gke.ManualTriggerKind)
	return obj
}

func gkePolicy() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(gke.APIVersion)
	obj.SetKind(gke.PolicyKind)
	return obj
}

func gkeStorageConfig() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(gke.APIVersion)
	obj.SetKind(gke.StorageConfigKind)
	return obj
}

func (r *LocalPodSnapshotShimReconciler) SetupWithManager(mgr ctrl.Manager) error {
	trigger := gkeManualTrigger()
	return ctrl.NewControllerManagedBy(mgr).
		For(trigger).
		Owns(&computev1.LocalRunscSnapshot{}).
		Complete(r)
}
