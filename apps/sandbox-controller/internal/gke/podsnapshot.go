package gke

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	"github.com/daytonaio/sandbox-controller/internal/render"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	APIVersion = "podsnapshot.gke.io/v1"

	StorageConfigKind   = "PodSnapshotStorageConfig"
	PolicyKind          = "PodSnapshotPolicy"
	ManualTriggerKind   = "PodSnapshotManualTrigger"
	PodSnapshotKind     = "PodSnapshot"
	PodSnapshotListKind = "PodSnapshotList"

	LabelSnapshotName = "compute.daytona.io/sandbox-snapshot"
)

func StorageConfig(snapshot *computev1.SandboxSnapshot) *unstructured.Unstructured {
	name := snapshot.Spec.GKE.StorageConfigName
	if name == "" {
		name = ObjectName("psc", snapshot.Name)
	}
	tokenSource := snapshot.Spec.GKE.Storage.TokenSource
	if tokenSource == "" {
		tokenSource = "podKSA"
	}

	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"snapshotStorageConfig": map[string]any{
					"gcs": map[string]any{
						"bucket":      snapshot.Spec.GKE.Storage.Bucket,
						"path":        snapshot.Spec.GKE.Storage.Path,
						"tokenSource": tokenSource,
					},
				},
			},
		},
	}
	obj.SetAPIVersion(APIVersion)
	obj.SetKind(StorageConfigKind)
	obj.SetName(name)
	obj.SetLabels(map[string]string{
		computev1.LabelManagedBy: computev1.ManagedByValue,
		LabelSnapshotName:        snapshot.Name,
	})
	return obj
}

func Policy(snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox) *unstructured.Unstructured {
	name := ObjectName("psp", snapshot.Name)
	postCheckpoint := string(snapshot.Spec.GKE.PostCheckpoint)
	if postCheckpoint == "" {
		postCheckpoint = string(computev1.PostCheckpointResume)
	}

	spec := map[string]any{
		"storageConfigName": StorageConfigName(snapshot),
		"selector": map[string]any{
			"matchLabels": map[string]any{
				computev1.LabelSandboxName: source.Name,
			},
		},
		"triggerConfig": map[string]any{
			"type":           "manual",
			"postCheckpoint": postCheckpoint,
		},
	}
	if snapshot.Spec.GKE.Retention != "" {
		spec["retentionConfig"] = map[string]any{
			"lastAccessTimeout": snapshot.Spec.GKE.Retention,
		}
	}

	obj := &unstructured.Unstructured{Object: map[string]any{"spec": spec}}
	obj.SetAPIVersion(APIVersion)
	obj.SetKind(PolicyKind)
	obj.SetName(name)
	obj.SetNamespace(snapshot.Namespace)
	obj.SetLabels(Labels(snapshot, source))
	return obj
}

func StorageConfigName(snapshot *computev1.SandboxSnapshot) string {
	if snapshot.Spec.GKE.StorageConfigName != "" {
		return snapshot.Spec.GKE.StorageConfigName
	}
	return ObjectName("psc", snapshot.Name)
}

func HasInlineStorage(snapshot *computev1.SandboxSnapshot) bool {
	return snapshot.Spec.GKE.Storage.Bucket != "" || snapshot.Spec.GKE.Storage.Path != ""
}

func ManualTrigger(snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox) *unstructured.Unstructured {
	name := ObjectName("pstmt", snapshot.Name)
	targetPod := source.Status.PodName
	if targetPod == "" {
		targetPod = render.PodName(source)
	}
	obj := &unstructured.Unstructured{
		Object: map[string]any{
			"spec": map[string]any{
				"targetPod": targetPod,
			},
		},
	}
	obj.SetAPIVersion(APIVersion)
	obj.SetKind(ManualTriggerKind)
	obj.SetName(name)
	obj.SetNamespace(snapshot.Namespace)
	obj.SetLabels(Labels(snapshot, source))
	return obj
}

func PodSnapshotList(_ string, snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox) *unstructured.UnstructuredList {
	list := &unstructured.UnstructuredList{}
	list.SetAPIVersion(APIVersion)
	list.SetKind(PodSnapshotListKind)
	return list
}

func Labels(snapshot *computev1.SandboxSnapshot, source *computev1.Sandbox) map[string]string {
	return map[string]string{
		computev1.LabelManagedBy:   computev1.ManagedByValue,
		computev1.LabelSandboxName: source.Name,
		LabelSnapshotName:          snapshot.Name,
	}
}

func IsReady(obj *unstructured.Unstructured) bool {
	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if !found {
		return false
	}
	for _, condition := range conditions {
		item, ok := condition.(map[string]any)
		if !ok {
			continue
		}
		if item["type"] == "Ready" && item["status"] == "True" {
			return true
		}
	}
	return false
}

func ObjectName(prefix string, name string) string {
	clean := strings.ToLower(name)
	clean = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, clean)
	clean = strings.Trim(clean, "-")
	if clean == "" {
		clean = "snapshot"
	}

	value := prefix + "-" + clean
	if len(value) <= 63 {
		return value
	}
	sum := sha256.Sum256([]byte(value))
	suffix := hex.EncodeToString(sum[:])[:10]
	maxPrefix := 63 - len(suffix) - 1
	return strings.Trim(value[:maxPrefix], "-") + "-" + suffix
}
