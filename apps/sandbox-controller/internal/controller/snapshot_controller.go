package controller

import (
	"context"
	"reflect"
	"time"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/gke"
	"github.com/daytonaio/sandbox-controller/internal/observability"
	"github.com/daytonaio/sandbox-controller/internal/render"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

type SandboxSnapshotReconciler struct {
	client.Client
	Scheme       *runtime.Scheme
	Recorder     record.EventRecorder
	StaleTimeout time.Duration
	Now          func() time.Time
}

func (r *SandboxSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	started := time.Now()
	defer func() { observability.ObserveReconcile("sandboxsnapshot", started, err) }()

	var snapshot computev1.SandboxSnapshot
	if err := r.Get(ctx, req.NamespacedName, &snapshot); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !snapshot.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &snapshot)
	}
	if !controllerutil.ContainsFinalizer(&snapshot, computev1.SandboxSnapshotFinalizer) {
		controllerutil.AddFinalizer(&snapshot, computev1.SandboxSnapshotFinalizer)
		return ctrl.Result{}, r.Update(ctx, &snapshot)
	}
	if r.isStaleSnapshot(&snapshot) {
		return r.reconcileStaleSnapshot(ctx, &snapshot)
	}

	provider := snapshot.Spec.Provider
	if provider == "" {
		provider = computev1.SnapshotProviderGKEPodSnapshot
	}
	if provider != computev1.SnapshotProviderGKEPodSnapshot && provider != computev1.SnapshotProviderLocalRunsc {
		phase := computev1.SandboxSnapshotPhaseFailed
		message := "Unsupported snapshot provider"
		return ctrl.Result{}, r.updateSnapshotStatus(ctx, &snapshot, SnapshotStatusPatch{
			Phase: &phase,
			Error: &message,
			Condition: condition(
				"ProviderReady",
				metav1.ConditionFalse,
				"UnsupportedProvider",
				message,
				snapshot.Generation,
			),
		})
	}

	var source computev1.Sandbox
	sourceKey := types.NamespacedName{Name: snapshot.Spec.Source.SandboxName, Namespace: snapshot.Namespace}
	if err := r.Get(ctx, sourceKey, &source); err != nil {
		phase := computev1.SandboxSnapshotPhasePending
		message := err.Error()
		return ctrl.Result{}, r.updateSnapshotStatus(ctx, &snapshot, SnapshotStatusPatch{
			Phase: &phase,
			Error: &message,
			Condition: condition(
				"SourceReady",
				metav1.ConditionFalse,
				"SourceNotFound",
				message,
				snapshot.Generation,
			),
		})
	}
	if render.HasPersistentVolumeClaim(&source) {
		phase := computev1.SandboxSnapshotPhaseFailed
		message := "PVC-backed sandbox state is unsupported for v1 snapshots"
		return ctrl.Result{}, r.updateSnapshotStatus(ctx, &snapshot, SnapshotStatusPatch{
			Phase: &phase,
			Error: &message,
			Condition: condition(
				"SourceSupported",
				metav1.ConditionFalse,
				"PersistentVolumeClaimUnsupported",
				message,
				snapshot.Generation,
			),
		})
	}

	compatibilityHash, templateName, err := r.snapshotCompatibility(ctx, &source)
	if err != nil {
		return ctrl.Result{}, err
	}

	switch provider {
	case computev1.SnapshotProviderGKEPodSnapshot:
		return r.reconcileGKESnapshot(ctx, &snapshot, &source, compatibilityHash, templateName)
	case computev1.SnapshotProviderLocalRunsc:
		return r.reconcileLocalRunscSnapshot(ctx, &snapshot, &source, compatibilityHash, templateName)
	default:
		return ctrl.Result{}, nil
	}
}

func (r *SandboxSnapshotReconciler) reconcileStaleSnapshot(ctx context.Context, snapshot *computev1.SandboxSnapshot) (ctrl.Result, error) {
	for _, obj := range r.snapshotOwnedObjects(snapshot) {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return ctrl.Result{}, err
		}
	}
	phase := computev1.SandboxSnapshotPhaseFailed
	message := "SandboxSnapshot exceeded stale pending/triggering timeout"
	return ctrl.Result{}, r.updateSnapshotStatus(ctx, snapshot, SnapshotStatusPatch{
		Phase: &phase,
		Error: &message,
		Condition: condition(
			"Ready",
			metav1.ConditionFalse,
			"StaleSnapshotTimeout",
			message,
			snapshot.Generation,
		),
	})
}

func (r *SandboxSnapshotReconciler) isStaleSnapshot(snapshot *computev1.SandboxSnapshot) bool {
	if r.StaleTimeout <= 0 {
		return false
	}
	switch snapshot.Status.Phase {
	case computev1.SandboxSnapshotPhasePending, computev1.SandboxSnapshotPhaseTriggering:
	default:
		return false
	}
	since := snapshot.CreationTimestamp.Time
	for _, condition := range snapshot.Status.Conditions {
		if condition.Type == "Ready" && !condition.LastTransitionTime.IsZero() {
			since = condition.LastTransitionTime.Time
			break
		}
	}
	if since.IsZero() {
		return false
	}
	return r.now().Sub(since) > r.StaleTimeout
}

func (r *SandboxSnapshotReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *SandboxSnapshotReconciler) reconcileDelete(ctx context.Context, snapshot *computev1.SandboxSnapshot) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(snapshot, computev1.SandboxSnapshotFinalizer) {
		return ctrl.Result{}, nil
	}
	for _, obj := range r.snapshotOwnedObjects(snapshot) {
		if err := r.Delete(ctx, obj); err != nil && !apierrors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return ctrl.Result{}, err
		}
	}
	controllerutil.RemoveFinalizer(snapshot, computev1.SandboxSnapshotFinalizer)
	return ctrl.Result{}, r.Update(ctx, snapshot)
}

func (r *SandboxSnapshotReconciler) snapshotOwnedObjects(snapshot *computev1.SandboxSnapshot) []client.Object {
	objects := []client.Object{}

	local := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: gke.ObjectName("lrs", snapshot.Name), Namespace: snapshot.Namespace},
	}
	objects = append(objects, local)

	for _, item := range []struct {
		kind      string
		name      string
		namespace string
	}{
		{kind: gke.PolicyKind, name: gke.ObjectName("psp", snapshot.Name), namespace: snapshot.Namespace},
		{kind: gke.ManualTriggerKind, name: gke.ObjectName("pstmt", snapshot.Name), namespace: snapshot.Namespace},
		{kind: gke.PodSnapshotKind, name: gke.ObjectName("ps", gke.ObjectName("pstmt", snapshot.Name)), namespace: snapshot.Namespace},
	} {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(gke.APIVersion)
		obj.SetKind(item.kind)
		obj.SetName(item.name)
		obj.SetNamespace(item.namespace)
		objects = append(objects, obj)
	}
	if gke.HasInlineStorage(snapshot) {
		obj := &unstructured.Unstructured{}
		obj.SetAPIVersion(gke.APIVersion)
		obj.SetKind(gke.StorageConfigKind)
		obj.SetName(gke.StorageConfigName(snapshot))
		objects = append(objects, obj)
	}
	return objects
}

func (r *SandboxSnapshotReconciler) snapshotCompatibility(ctx context.Context, source *computev1.Sandbox) (string, string, error) {
	if source.Spec.TemplateName != "" {
		var template computev1.SandboxTemplate
		if err := r.Get(ctx, types.NamespacedName{Name: source.Spec.TemplateName, Namespace: source.Namespace}, &template); err != nil {
			return "", "", err
		}
		if template.Status.CompatibilityHash != "" {
			return template.Status.CompatibilityHash, template.Name, nil
		}
		hash, err := render.CompatibilityHash(&computev1.Sandbox{
			ObjectMeta: metav1.ObjectMeta{Name: template.Name, Namespace: template.Namespace},
			Spec:       template.Spec.Template,
		})
		return hash, template.Name, err
	}
	compatibilityHash := source.Status.SpecHash
	if compatibilityHash == "" {
		hash, err := render.CompatibilityHash(source)
		if err != nil {
			return "", "", err
		}
		compatibilityHash = hash
	}
	return compatibilityHash, "", nil
}

func (r *SandboxSnapshotReconciler) reconcileGKESnapshot(ctx context.Context, snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox, compatibilityHash string, templateName string) (ctrl.Result, error) {
	if err := r.ensureGKEResources(ctx, snapshot, source); err != nil {
		return ctrl.Result{}, err
	}

	providerObjectName, storageRef, ready, err := r.findReadyGKEPodSnapshot(ctx, snapshot, source)
	if err != nil && !meta.IsNoMatchError(err) {
		return ctrl.Result{}, err
	}

	phase := computev1.SandboxSnapshotPhaseTriggering
	readyStatus := metav1.ConditionFalse
	reason := "WaitingForGKEPodSnapshot"
	message := "Waiting for GKE to create a Ready PodSnapshot"
	if ready {
		phase = computev1.SandboxSnapshotPhaseReady
		readyStatus = metav1.ConditionTrue
		reason = "Ready"
		message = "GKE PodSnapshot is ready for restore"
	}

	result := ctrl.Result{}
	if !ready {
		result.RequeueAfter = 2 * time.Second
	}
	return result, r.updateSnapshotStatus(ctx, snapshot, SnapshotStatusPatch{
		Phase:              &phase,
		ProviderObjectName: &providerObjectName,
		StorageRef:         &storageRef,
		PolicyName:         ptr(gke.ObjectName("psp", snapshot.Name)),
		TriggerName:        ptr(gke.ObjectName("pstmt", snapshot.Name)),
		SourcePodName:      ptr(source.Status.PodName),
		TemplateName:       &templateName,
		CompatibilityHash:  &compatibilityHash,
		Error:              ptr(""),
		Condition: condition(
			"Ready",
			readyStatus,
			reason,
			message,
			snapshot.Generation,
		),
	})
}

func (r *SandboxSnapshotReconciler) reconcileLocalRunscSnapshot(ctx context.Context, snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox, compatibilityHash string, templateName string) (ctrl.Result, error) {
	sourcePodName := source.Status.PodName
	if sourcePodName == "" {
		sourcePodName = render.PodName(source)
	}

	var sourcePod corev1.Pod
	if err := r.Get(ctx, types.NamespacedName{Name: sourcePodName, Namespace: source.Namespace}, &sourcePod); err != nil {
		if apierrors.IsNotFound(err) {
			phase := computev1.SandboxSnapshotPhasePending
			message := "Source Pod is not available for local runsc checkpoint"
			return ctrl.Result{}, r.updateSnapshotStatus(ctx, snapshot, SnapshotStatusPatch{
				Phase:             &phase,
				SourcePodName:     &sourcePodName,
				TemplateName:      &templateName,
				CompatibilityHash: &compatibilityHash,
				Error:             &message,
				Condition: condition(
					"SourcePodReady",
					metav1.ConditionFalse,
					"SourcePodNotFound",
					message,
					snapshot.Generation,
				),
			})
		}
		return ctrl.Result{}, err
	}

	if sourcePod.Spec.NodeName == "" {
		phase := computev1.SandboxSnapshotPhasePending
		message := "Source Pod is not scheduled onto a node"
		return ctrl.Result{}, r.updateSnapshotStatus(ctx, snapshot, SnapshotStatusPatch{
			Phase:             &phase,
			SourcePodName:     &sourcePodName,
			TemplateName:      &templateName,
			CompatibilityHash: &compatibilityHash,
			Error:             &message,
			Condition: condition(
				"SourcePodReady",
				metav1.ConditionFalse,
				"SourcePodNotScheduled",
				message,
				snapshot.Generation,
			),
		})
	}

	request, err := r.ensureLocalRunscSnapshot(ctx, snapshot, source, &sourcePod)
	if err != nil {
		return ctrl.Result{}, err
	}

	phase := computev1.SandboxSnapshotPhaseTriggering
	readyStatus := metav1.ConditionFalse
	reason := "WaitingForLocalRunscSnapshot"
	message := "Waiting for local runsc snapshot request to complete"
	providerObjectName := request.Name
	storageRef := request.Status.StorageRef
	errorMessage := request.Status.Error

	switch request.Status.Phase {
	case computev1.LocalRunscSnapshotPhaseReady:
		phase = computev1.SandboxSnapshotPhaseReady
		readyStatus = metav1.ConditionTrue
		reason = "Ready"
		message = "Local runsc snapshot is ready for restore"
	case computev1.LocalRunscSnapshotPhaseFailed:
		phase = computev1.SandboxSnapshotPhaseFailed
		reason = "LocalRunscSnapshotFailed"
		if errorMessage != "" {
			message = errorMessage
		} else {
			message = "Local runsc snapshot request failed"
		}
	case "":
		phase = computev1.SandboxSnapshotPhasePending
	}

	return ctrl.Result{}, r.updateSnapshotStatus(ctx, snapshot, SnapshotStatusPatch{
		Phase:              &phase,
		ProviderObjectName: &providerObjectName,
		SourcePodName:      &sourcePodName,
		TemplateName:       &templateName,
		CompatibilityHash:  &compatibilityHash,
		StorageRef:         &storageRef,
		Error:              &errorMessage,
		Condition: condition(
			"Ready",
			readyStatus,
			reason,
			message,
			snapshot.Generation,
		),
	})
}

func (r *SandboxSnapshotReconciler) ensureLocalRunscSnapshot(ctx context.Context, snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox, sourcePod *corev1.Pod) (*computev1.LocalRunscSnapshot, error) {
	desired := &computev1.LocalRunscSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gke.ObjectName("lrs", snapshot.Name),
			Namespace: snapshot.Namespace,
			Labels: map[string]string{
				computev1.LabelManagedBy:   computev1.ManagedByValue,
				computev1.LabelSandboxName: source.Name,
				gke.LabelSnapshotName:      snapshot.Name,
			},
		},
		Spec: computev1.LocalRunscSnapshotSpec{
			SandboxName:         source.Name,
			SourcePodName:       sourcePod.Name,
			SourceContainerName: render.WorkloadContainerName,
			NodeName:            sourcePod.Spec.NodeName,
			Storage:             snapshot.Spec.Local.Storage,
		},
	}
	if err := controllerutil.SetControllerReference(snapshot, desired, r.Scheme); err != nil {
		return nil, err
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
		if err := r.Update(ctx, &existing); err != nil {
			return nil, err
		}
	}
	return &existing, nil
}

func (r *SandboxSnapshotReconciler) ensureGKEResources(ctx context.Context, snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox) error {
	if gke.HasInlineStorage(snapshot) {
		storageConfig := gke.StorageConfig(snapshot)
		if err := r.createOrPatchUnstructured(ctx, storageConfig); err != nil {
			return err
		}
	}

	policy := gke.Policy(snapshot, source)
	if err := controllerutil.SetControllerReference(snapshot, policy, r.Scheme); err != nil {
		return err
	}
	if err := r.createOrPatchUnstructured(ctx, policy); err != nil {
		return err
	}

	trigger := gke.ManualTrigger(snapshot, source)
	if err := controllerutil.SetControllerReference(snapshot, trigger, r.Scheme); err != nil {
		return err
	}
	return r.createOrPatchUnstructured(ctx, trigger)
}

func (r *SandboxSnapshotReconciler) createOrPatchUnstructured(ctx context.Context, desired *unstructured.Unstructured) error {
	var existing unstructured.Unstructured
	existing.SetAPIVersion(desired.GetAPIVersion())
	existing.SetKind(desired.GetKind())

	key := types.NamespacedName{Name: desired.GetName(), Namespace: desired.GetNamespace()}
	if err := r.Get(ctx, key, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return r.Create(ctx, desired)
		}
		if meta.IsNoMatchError(err) {
			return nil
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
		return r.Update(ctx, &existing)
	}
	return nil
}

func (r *SandboxSnapshotReconciler) findReadyGKEPodSnapshot(ctx context.Context, snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox) (string, string, bool, error) {
	trigger := &unstructured.Unstructured{}
	trigger.SetAPIVersion(gke.APIVersion)
	trigger.SetKind(gke.ManualTriggerKind)
	triggerKey := types.NamespacedName{Name: gke.ObjectName("pstmt", snapshot.Name), Namespace: snapshot.Namespace}
	if err := r.Get(ctx, triggerKey, trigger); err == nil {
		snapshotCreated, _, _ := unstructured.NestedString(trigger.Object, "status", "snapshotCreated")
		if snapshotCreated != "" {
			return r.gkePodSnapshotReady(ctx, snapshot.Namespace, snapshotCreated)
		}
	} else if !apierrors.IsNotFound(err) {
		return "", "", false, err
	}

	list := gke.PodSnapshotList(snapshot.Namespace, snapshot, source)
	if err := r.List(ctx, list, client.InNamespace(snapshot.Namespace)); err != nil {
		return "", "", false, err
	}

	policyName := gke.ObjectName("psp", snapshot.Name)
	for i := range list.Items {
		item := &list.Items[i]
		itemPolicyName, _, _ := unstructured.NestedString(item.Object, "spec", "policyName")
		if itemPolicyName != policyName {
			continue
		}
		if gke.IsReady(item) {
			storageRef, _, _ := unstructured.NestedString(item.Object, "status", "artifactStorageRef")
			return item.GetName(), storageRef, true, nil
		}
	}
	if len(list.Items) > 0 {
		for i := range list.Items {
			itemPolicyName, _, _ := unstructured.NestedString(list.Items[i].Object, "spec", "policyName")
			if itemPolicyName == policyName {
				return list.Items[i].GetName(), "", false, nil
			}
		}
	}
	return "", "", false, nil
}

func (r *SandboxSnapshotReconciler) gkePodSnapshotReady(ctx context.Context, namespace string, name string) (string, string, bool, error) {
	podSnapshot := &unstructured.Unstructured{}
	podSnapshot.SetAPIVersion(gke.APIVersion)
	podSnapshot.SetKind(gke.PodSnapshotKind)
	if err := r.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, podSnapshot); err != nil {
		if apierrors.IsNotFound(err) {
			return name, "", false, nil
		}
		return "", "", false, err
	}
	storageRef, _, _ := unstructured.NestedString(podSnapshot.Object, "status", "artifactStorageRef")
	return name, storageRef, gke.IsReady(podSnapshot), nil
}

type SnapshotStatusPatch struct {
	Phase              *computev1.SandboxSnapshotPhase
	ProviderObjectName *string
	PolicyName         *string
	TriggerName        *string
	SourcePodName      *string
	TemplateName       *string
	CompatibilityHash  *string
	StorageRef         *string
	Error              *string
	Condition          *metav1.Condition
}

func (r *SandboxSnapshotReconciler) updateSnapshotStatus(ctx context.Context, snapshot *computev1.SandboxSnapshot, patch SnapshotStatusPatch) error {
	next := snapshot.DeepCopyObject().(*computev1.SandboxSnapshot)
	next.Status.ObservedGeneration = snapshot.Generation
	if patch.Phase != nil {
		next.Status.Phase = *patch.Phase
	}
	if patch.ProviderObjectName != nil {
		next.Status.ProviderObjectName = *patch.ProviderObjectName
	}
	if patch.PolicyName != nil {
		next.Status.PolicyName = *patch.PolicyName
	}
	if patch.TriggerName != nil {
		next.Status.TriggerName = *patch.TriggerName
	}
	if patch.SourcePodName != nil {
		next.Status.SourcePodName = *patch.SourcePodName
	}
	if patch.TemplateName != nil {
		next.Status.TemplateName = *patch.TemplateName
	}
	if patch.CompatibilityHash != nil {
		next.Status.CompatibilityHash = *patch.CompatibilityHash
	}
	if patch.StorageRef != nil {
		next.Status.StorageRef = *patch.StorageRef
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
	if err := r.Status().Update(ctx, next); err != nil {
		return err
	}
	if patch.Phase != nil {
		observability.SnapshotPhase.WithLabelValues(snapshot.Namespace, snapshot.Name, string(*patch.Phase)).Set(1)
		if r.Recorder != nil && snapshot.Status.Phase != *patch.Phase {
			r.Recorder.Eventf(snapshot, corev1.EventTypeNormal, "SandboxSnapshotPhaseChanged", "SandboxSnapshot phase changed from %s to %s", snapshot.Status.Phase, *patch.Phase)
		}
	}
	return nil
}

func (r *SandboxSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&computev1.SandboxSnapshot{}).
		Owns(&computev1.LocalRunscSnapshot{}).
		Complete(r)
}
