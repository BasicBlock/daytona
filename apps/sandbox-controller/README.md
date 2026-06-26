# Daytona Sandbox Controller

This is the greenfield Kubernetes compute substrate. It intentionally does not preserve the old runner abstraction.

The controller owns product CRDs under `compute.daytona.io/v1alpha1`:

- `Sandbox` reconciles to one gVisor Pod and one stable Service.
- `SandboxSnapshot` triggers provider-specific checkpoint resources.
- `SandboxTemplate` is reserved for immutable compatibility templates.
- `LocalRunscSnapshot` is a node-agent request object for the local VM test bed.

The production snapshot provider is GKE Pod Snapshots. The controller creates GKE-shaped `PodSnapshotPolicy` and `PodSnapshotManualTrigger` objects as unstructured resources, then records the ready `PodSnapshot` name in `SandboxSnapshot.status.providerObjectName`.

No runner, runner job, runner draining, or hidden runner capacity model exists in this app. The Kubernetes scheduler is the compute plane.

## Binaries

The image contains four entrypoints:

- `/sandbox-controller`: controller-runtime manager for `Sandbox` and `SandboxSnapshot`.
- `/sandbox-api`: thin HTTP facade over the CRDs.
- `/local-node-agent`: local-only DaemonSet agent for raw `runsc checkpoint` smoke work.
- `/toolbox-sidecar`: stateless sandbox sidecar used by rendered Sandbox Pods.

## Install

Apply the base controller/API stack:

```bash
kubectl apply -k apps/sandbox-controller/config
```

The base install creates the CRDs, controller deployment, API deployment, RBAC, and the `daytona-system` namespace. Sandbox workloads are created in the configured sandbox namespace, which defaults to `sandboxes` for the API.

Build the image locally:

```bash
docker build -f apps/sandbox-controller/Dockerfile -t ghcr.io/daytonaio/sandbox-controller:dev .
```

Run the controller against the current kubeconfig during development:

```bash
nix develop .#go --command bash -c "cd apps/sandbox-controller && go run ./cmd/sandbox-controller --default-toolbox-image ghcr.io/daytonaio/sandbox-controller:dev"
```

Run the API facade locally:

```bash
nix develop .#go --command bash -c "cd apps/sandbox-controller && go run ./cmd/sandbox-api --listen :8090 --namespace sandboxes"
```

## REST Facade

The HTTP API maps directly to CRDs:

```bash
curl -X POST localhost:8090/sandboxes \
  -H 'content-type: application/json' \
  -d '{"name":"ubuntu-agent","spec":{"image":"ubuntu:24.04","command":["sleep"],"args":["infinity"]}}'

curl -X POST localhost:8090/sandboxes/ubuntu-agent:snapshot \
  -H 'content-type: application/json' \
  -d '{"name":"ubuntu-agent-warm","provider":"GKEPodSnapshot","gke":{"storageConfigName":"daytona-podsnapshots","postCheckpoint":"resume"}}'

curl -X POST localhost:8090/sandboxes/ubuntu-agent:fork \
  -H 'content-type: application/json' \
  -d '{"name":"ubuntu-agent-fork","snapshotName":"ubuntu-agent-warm"}'

curl -X POST localhost:8090/sandboxes/ubuntu-agent:stop \
  -H 'content-type: application/json' \
  -d '{"snapshotBeforeStop":true,"snapshotName":"ubuntu-agent-stop","provider":"GKEPodSnapshot"}'

curl "localhost:8090/sandboxes/ubuntu-agent/logs?tailLines=100&timestamps=true"
```

The API intentionally has no runner endpoints.

## Local Test Bed

Use the checked-in Lima/k3s test bed rather than Docker Desktop Kubernetes:

```bash
limactl start --name daytona-sandbox apps/sandbox-controller/hack/local-provider/daytona-sandbox.lima.yaml
apps/sandbox-controller/hack/local-provider/preflight.sh daytona-sandbox
```

The local stack installs k3s/containerd, gVisor `runsc`, `RuntimeClass: gvisor`, MinIO, the base controller/API stack, the local node-agent, and the local PodSnapshot shim. See [docs/local-test-bed.md](docs/local-test-bed.md).

Install only the local add-on resources after the base stack:

```bash
kubectl apply -k apps/sandbox-controller/config/local
```

The local overlay runs `/local-node-agent` as a privileged DaemonSet. It watches `LocalRunscSnapshot` objects assigned to its node and runs:

```bash
runsc checkpoint --image-path <artifact-path> <sandbox-id>
```

Raw smoke endpoints are exposed inside each node-agent Pod:

```bash
curl -X POST http://<agent-pod-ip>:2281/checkpoint \
  -H 'content-type: application/json' \
  -d '{"namespace":"sandboxes","name":"manual","sandboxId":"sandbox-ubuntu-agent","imagePath":"/var/lib/daytona-localrunsc/manual"}'

curl -X POST http://<agent-pod-ip>:2281/restore \
  -H 'content-type: application/json' \
  -d '{"sandboxId":"sandbox-ubuntu-agent","restoredSandboxId":"sandbox-ubuntu-agent-restore","imagePath":"/var/lib/daytona-localrunsc/manual"}'
```

Create a local-provider snapshot through the product CRD:

```bash
kubectl apply -f apps/sandbox-controller/config/samples/sandboxsnapshot-localrunsc.yaml
```

The product-facing local E2E path uses `SandboxSnapshot` plus GKE-shaped `PodSnapshotPolicy`/`PodSnapshotManualTrigger` resources. `LocalRunscSnapshot` is an internal node-agent request object. GKE remains the source of truth for production restore behavior.

## GKE Requirements

For production restore tests, use a GKE cluster with Pod Snapshots enabled, GKE Sandbox/gVisor node support, Workload Identity Federation, and a Cloud Storage bucket configured through `PodSnapshotStorageConfig`.

The stg-cluster E2E target is `stg-cluster-operator.tail9212cd.ts.net`; preflight refuses any other context. See [docs/gke-e2e.md](docs/gke-e2e.md).

Apply a storage config sample after replacing bucket/path values:

```bash
kubectl apply -f apps/sandbox-controller/config/samples/gke-podsnapshot-storageconfig.yaml
```

Then create a sandbox and GKE snapshot:

```bash
kubectl apply -f apps/sandbox-controller/config/samples/sandbox.yaml
kubectl apply -f apps/sandbox-controller/config/samples/sandboxsnapshot-gke.yaml
kubectl get sandboxsnapshots -n sandboxes
```

Restore/fork uses the ready `SandboxSnapshot.status.providerObjectName`; the rendered Pod receives `podsnapshot.gke.io/ps-name`. GKE Pod Snapshots define v1 product semantics: process memory, rootfs changes, `emptyDir`, and `tmpfs` are snapshotable; persistent volume contents are not part of v1 sandbox state.

## Toolbox Sidecar

The rendered Sandbox Pod includes a workload container and `/toolbox-sidecar`. The sidecar is checkpoint-safe by design:

- identity is reloaded from environment on readiness checks;
- no external connection state is stored across restore;
- `/healthz`, `/readyz`, and `/identity` are implemented;
- `/exec`, `/files`, `/ports`, and `/ssh` are implemented as JSON control endpoints. `/exec` enters the workload process namespace with `nsenter` when running in the rendered Pod.

## Semantics And Secrets

Snapshot/restore compatibility, PVC rejection, and template matching are documented in [docs/restore-semantics.md](docs/restore-semantics.md).

Doppler secret selection and managed Secret injection are documented in [docs/doppler.md](docs/doppler.md).

## Verify

Run unit tests:

```bash
nix --extra-experimental-features nix-command --extra-experimental-features flakes develop .#go --command bash -c "cd apps/sandbox-controller && go test ./..."
```

Render base manifests:

```bash
kubectl kustomize apps/sandbox-controller/config >/tmp/sandbox-controller.yaml
```

Render the local node-agent overlay:

```bash
kubectl kustomize apps/sandbox-controller/config/local >/tmp/sandbox-controller-local.yaml
```

Compile and run opt-in cluster tests in skip mode:

```bash
nix --extra-experimental-features nix-command --extra-experimental-features flakes develop .#go --command bash -c "cd apps/sandbox-controller && go test -tags e2e ./test/e2e"
```

Run real cluster lifecycle tests:

```bash
DAYTONA_E2E=1 \
nix --extra-experimental-features nix-command --extra-experimental-features flakes develop .#go --command bash -c "cd apps/sandbox-controller && go test -tags e2e ./test/e2e -run TestSandboxLifecycleE2E"
```

Run local VM runsc tests against an existing running sandbox:

```bash
DAYTONA_LOCAL_RUNSC_E2E=1 \
DAYTONA_LOCAL_RUNSC_SOURCE_SANDBOX=ubuntu-agent \
nix --extra-experimental-features nix-command --extra-experimental-features flakes develop .#go --command bash -c "cd apps/sandbox-controller && go test -tags e2e ./test/e2e -run TestLocalRunscSnapshotE2E"
```

Run GKE PodSnapshot tests against an existing running sandbox and storage config. This creates a snapshot, waits for it to become ready, creates a restored forked `Sandbox`, waits for it to run, and verifies the restored Pod has the `podsnapshot.gke.io/ps-name` annotation:

```bash
DAYTONA_GKE_E2E=1 \
DAYTONA_GKE_SOURCE_SANDBOX=ubuntu-agent \
DAYTONA_GKE_PODSNAPSHOT_STORAGE_CONFIG=daytona-podsnapshots \
nix --extra-experimental-features nix-command --extra-experimental-features flakes develop .#go --command bash -c "cd apps/sandbox-controller && go test -tags e2e ./test/e2e -run TestGKEPodSnapshotRestoreE2E"
```
