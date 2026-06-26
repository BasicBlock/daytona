# Local Lima/k3s Test Bed

The local provider uses an Ubuntu 24.04 Lima VM with k3s, containerd, gVisor `runsc`, `RuntimeClass: gvisor`, MinIO, and the local PodSnapshot shim.

## Lifecycle

```bash
limactl start --name daytona-sandbox apps/sandbox-controller/hack/local-provider/daytona-sandbox.lima.yaml
apps/sandbox-controller/hack/local-provider/load-dev-image.sh daytona-sandbox
apps/sandbox-controller/hack/local-provider/preflight.sh daytona-sandbox
limactl stop daytona-sandbox
limactl delete daytona-sandbox
```

The Lima template runs `hack/local-provider/provision-vm.sh`, which installs `runsc`, k3s/containerd, configures the `gvisor` runtime handler, applies the base controller stack, applies `config/local`, and patches the controller with `--enable-local-podsnapshot-shim`. Use `load-dev-image.sh` after source changes to build the multi-entrypoint image, import it into k3s containerd, and restart the controller/API/node-agent.

## Raw Runtime Smoke

```bash
apps/sandbox-controller/hack/local-provider/raw-runsc-smoke.sh daytona-sandbox
apps/sandbox-controller/hack/local-provider/minio-smoke.sh daytona-sandbox
```

`raw-runsc-smoke.sh` validates `runsc checkpoint` and `runsc restore` outside the CRD flow. `minio-smoke.sh` validates upload/download through the local MinIO service.

## Local CRD Flow

The local product-facing path is the same shape as GKE:

1. Create a `Sandbox`.
2. Create a `SandboxSnapshot` with provider `GKEPodSnapshot`.
3. The controller creates `PodSnapshotPolicy` and `PodSnapshotManualTrigger`.
4. The local shim watches the GKE-shaped trigger and creates an internal `LocalRunscSnapshot`.
5. The node-agent runs `runsc checkpoint`, uploads artifacts to MinIO, and records `storageRef`.
6. The shim writes a ready GKE-shaped `PodSnapshot`.

`LocalRunscSnapshot` is internal and should not be the local E2E product API.

## Cleanup

```bash
apps/sandbox-controller/hack/local-provider/cleanup-local-artifacts.sh daytona-sandbox
```

Cleanup deletes test CRs in `sandboxes` and removes `/var/lib/daytona-localrunsc/*` inside the VM. `LocalRunscSnapshot` also has a finalizer that deletes node-local and MinIO artifacts when request objects are deleted.
