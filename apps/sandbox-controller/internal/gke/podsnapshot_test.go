package gke

import (
	"strings"
	"testing"

	computev1 "github.com/daytonaio/sandbox-controller/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPolicyUsesManualTriggerAndSandboxSelector(t *testing.T) {
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "sandboxes"},
		Spec: computev1.SandboxSnapshotSpec{
			GKE: computev1.GKEPodSnapshotSpec{
				StorageConfigName: "storage",
				PostCheckpoint:    computev1.PostCheckpointStop,
				Retention:         "7d",
			},
		},
	}
	source := &computev1.Sandbox{ObjectMeta: metav1.ObjectMeta{Name: "agent", Namespace: "sandboxes"}}

	policy := Policy(snapshot, source)
	spec := policy.Object["spec"].(map[string]any)
	trigger := spec["triggerConfig"].(map[string]any)

	if trigger["type"] != "manual" {
		t.Fatalf("expected manual trigger, got %v", trigger["type"])
	}
	if trigger["postCheckpoint"] != "stop" {
		t.Fatalf("expected postCheckpoint stop, got %v", trigger["postCheckpoint"])
	}
	selector := spec["selector"].(map[string]any)["matchLabels"].(map[string]any)
	if selector[computev1.LabelSandboxName] != "agent" {
		t.Fatalf("expected sandbox selector, got %#v", selector)
	}
}

func TestStorageConfigUsesInlineGCSConfig(t *testing.T) {
	snapshot := &computev1.SandboxSnapshot{
		ObjectMeta: metav1.ObjectMeta{Name: "warm", Namespace: "sandboxes"},
		Spec: computev1.SandboxSnapshotSpec{
			GKE: computev1.GKEPodSnapshotSpec{
				StorageConfigName: "storage",
				Storage: computev1.GKEPodSnapshotStorage{
					Bucket:      "bucket",
					Path:        "snapshots",
					TokenSource: "podKSA",
				},
			},
		},
	}

	config := StorageConfig(snapshot)
	spec := config.Object["spec"].(map[string]any)
	storage := spec["snapshotStorageConfig"].(map[string]any)
	gcs := storage["gcs"].(map[string]any)

	if gcs["bucket"] != "bucket" || gcs["path"] != "snapshots" {
		t.Fatalf("unexpected gcs config: %#v", gcs)
	}
}

func TestObjectNameIsDNSLabelSized(t *testing.T) {
	name := ObjectName("pstmt", strings.Repeat("a", 100))
	if len(name) > 63 {
		t.Fatalf("expected name <= 63 chars, got %d", len(name))
	}
	if strings.HasSuffix(name, "-") {
		t.Fatalf("expected trimmed name, got %q", name)
	}
}
