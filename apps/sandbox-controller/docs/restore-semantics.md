# Snapshot And Restore Semantics

GKE PodSnapshot is the production source of truth. LocalRunsc exists only to make the Lima/k3s test bed mimic the GKE policy/manual-trigger/readiness flow.

## Captured State

The v1 contract is compatible with GKE Pod Snapshots: process memory, writable container rootfs state, `emptyDir`, and tmpfs-style state are snapshotable. PVC-backed state is not part of v1 sandbox restore.

The controller rejects snapshots and restores for sandboxes with `spec.volumes[].persistentVolumeClaim` and surfaces a `PersistentVolumeClaimUnsupported` condition.

## Compatibility

`SandboxTemplate` is the canonical restore contract when `Sandbox.spec.templateName` is set.

- `SandboxTemplate.spec` is immutable by CRD validation.
- The template controller stores `status.compatibilityHash`.
- `SandboxSnapshot.status.templateName` and `status.compatibilityHash` are recorded when the source sandbox uses a template.
- Restore fails when the requested sandbox template name does not match the snapshot template.
- Restore also fails when the requested sandbox spec hash does not match the template hash.

Without a template, compatibility falls back to the sandbox spec hash.

## Provider Differences

GKE uses native `podsnapshot.gke.io/v1` resources and restores Pods with the `podsnapshot.gke.io/ps-name` annotation. The local shim writes GKE-shaped resources but fulfills them through `runsc checkpoint` and MinIO artifacts. Local restore is limited to validating provider flow and artifact handling; production restore behavior remains GKE-defined.
