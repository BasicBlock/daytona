# stg-cluster GKE E2E

The GKE E2E target is the current kubeconfig context:

```text
stg-cluster-operator.tail9212cd.ts.net
```

The preflight refuses to run destructive tests on any other context.

## Required Cluster State

- GKE Pod Snapshot CRDs:
  - `PodSnapshotStorageConfig`
  - `PodSnapshotPolicy`
  - `PodSnapshotManualTrigger`
  - `PodSnapshot`
- `RuntimeClass: gvisor`.
- A GKE Sandbox/gVisor-capable node pool.
- Compatible node machine types with `node.kubernetes.io/instance-type`.
- Workload Identity Federation configured for Pod Snapshot storage access.
- The sandbox controller image under test deployed in `daytona-system`.
- A staging Cloud Storage bucket supplied by CI/Doppler.

## Safety Rules

Tests use `daytona-sandbox-e2e` by default. Every test-created resource has:

```text
compute.daytona.io/e2e=true
compute.daytona.io/e2e-run-id=<unique run id>
```

Cleanup must only delete resources in the E2E namespace with the run ID. Tests must not delete shared namespaces, buckets, RuntimeClasses, node pools, or cluster-scoped GKE PodSnapshot CRDs.

## CI Variables

```text
STG_CLUSTER_KUBECONFIG
DAYTONA_GKE_STORAGE_BUCKET
DAYTONA_GKE_STORAGE_PREFIX
DAYTONA_API_BASE_URL
DAYTONA_ACCEPTANCE_STORAGE_CONFIG
```

The GKE restore test creates a temporary `PodSnapshotStorageConfig` using the configured bucket and a run-ID prefix, creates its own source sandbox, snapshots it, restores the latest ready snapshot, and verifies the restored Pod annotation and runtime class.
