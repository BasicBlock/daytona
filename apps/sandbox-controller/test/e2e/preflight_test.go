//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	stgContextName = "stg-cluster-operator.tail9212cd.ts.net"

	e2eLabelKey      = "compute.daytona.io/e2e"
	e2eRunIDLabelKey = "compute.daytona.io/e2e-run-id"
)

func TestGKEPreflightE2E(t *testing.T) {
	if os.Getenv("DAYTONA_GKE_E2E") != "1" {
		t.Skip("set DAYTONA_GKE_E2E=1 to run GKE preflight tests")
	}
	ctx := context.Background()
	k8sClient := e2eClient(t)

	requireCurrentContext(t, stgContextName)
	requireObject(t, ctx, k8sClient, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "podsnapshots.podsnapshot.gke.io")
	requireObject(t, ctx, k8sClient, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "podsnapshotpolicies.podsnapshot.gke.io")
	requireObject(t, ctx, k8sClient, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "podsnapshotmanualtriggers.podsnapshot.gke.io")
	requireObject(t, ctx, k8sClient, "apiextensions.k8s.io/v1", "CustomResourceDefinition", "", "podsnapshotstorageconfigs.podsnapshot.gke.io")
	requireObject(t, ctx, k8sClient, "node.k8s.io/v1", "RuntimeClass", "", "gvisor")
	requireObject(t, ctx, k8sClient, "apps/v1", "Deployment", "daytona-system", "sandbox-controller")
	requireGvisorNodePool(t, ctx, k8sClient)
	bucket := requireEnv(t, "DAYTONA_GKE_STORAGE_BUCKET")
	prefix := requireEnv(t, "DAYTONA_GKE_STORAGE_PREFIX")
	if storageConfigName := os.Getenv("DAYTONA_GKE_STORAGE_CONFIG"); storageConfigName != "" {
		requireStorageConfigTarget(t, ctx, k8sClient, storageConfigName, bucket, prefix)
	}
}

func requireCurrentContext(t *testing.T, want string) {
	t.Helper()
	config, err := clientcmd.NewDefaultClientConfigLoadingRules().Load()
	if err != nil {
		t.Fatal(err)
	}
	if config.CurrentContext != want {
		t.Fatalf("refusing to run destructive GKE E2E on context %q; expected %q", config.CurrentContext, want)
	}
}

func requireObject(t *testing.T, ctx context.Context, k8sClient client.Client, apiVersion string, kind string, namespace string, name string) *unstructured.Unstructured {
	t.Helper()
	obj := &unstructured.Unstructured{}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		t.Fatal(err)
	}
	obj.SetGroupVersionKind(gv.WithKind(kind))
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, obj); err != nil {
		t.Fatalf("required %s %s/%s is missing: %v", kind, namespace, name, err)
	}
	return obj
}

func requireGvisorNodePool(t *testing.T, ctx context.Context, k8sClient client.Client) {
	t.Helper()
	var nodes corev1.NodeList
	if err := k8sClient.List(ctx, &nodes); err != nil {
		t.Fatal(err)
	}
	for _, node := range nodes.Items {
		labels := node.GetLabels()
		if labels["cloud.google.com/gke-sandbox"] == "true" || labels["sandbox.gke.io/runtime"] == "gvisor" {
			instanceType := labels["node.kubernetes.io/instance-type"]
			if instanceType == "" {
				t.Fatalf("gVisor node %s is missing node.kubernetes.io/instance-type", node.Name)
			}
			return
		}
	}
	t.Fatalf("no GKE Sandbox/gVisor-capable node pool found; inspect with: kubectl get nodes -L cloud.google.com/gke-sandbox,cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type")
}

func requireStorageConfigTarget(t *testing.T, ctx context.Context, k8sClient client.Client, name string, bucket string, prefix string) {
	t.Helper()
	obj := requireObject(t, ctx, k8sClient, "podsnapshot.gke.io/v1", "PodSnapshotStorageConfig", "", name)
	gotBucket, _, _ := unstructured.NestedString(obj.Object, "spec", "snapshotStorageConfig", "gcs", "bucket")
	gotPath, _, _ := unstructured.NestedString(obj.Object, "spec", "snapshotStorageConfig", "gcs", "path")
	if gotBucket != bucket {
		t.Fatalf("PodSnapshotStorageConfig %s bucket = %q, expected %q", name, gotBucket, bucket)
	}
	if gotPath != prefix {
		t.Fatalf("PodSnapshotStorageConfig %s path = %q, expected %q", name, gotPath, prefix)
	}
}

func requireEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value == "" {
		t.Fatalf("%s is required", name)
	}
	return value
}

func e2eLabels(t *testing.T) map[string]string {
	t.Helper()
	return map[string]string{
		e2eLabelKey:      "true",
		e2eRunIDLabelKey: e2eRunID(t),
	}
}

func e2eRunID(t *testing.T) string {
	t.Helper()
	if value := os.Getenv("DAYTONA_E2E_RUN_ID"); value != "" {
		return value
	}
	return fmt.Sprintf("run-%d", os.Getpid())
}
