# Greenfield Kubernetes Compute Substrate: Remaining Work

This checklist tracks what is still incomplete in the Kubernetes-native sandbox controller implementation. The current scaffold already has CRDs, basic controllers, a REST facade, a GKE PodSnapshot integration path, a LocalRunsc node-agent scaffold, and unit-level coverage.

Status note: local Lima/k3s provider, raw runsc checkpoint/restore, MinIO artifact storage, API acceptance, unit tests, envtest, and live `stg-cluster` GKE preflight/restore validation are implemented and verified.

## Local Provider And Test Bed

- [x] Provision the local Ubuntu VM test bed with Lima.
- [x] Add a checked-in Lima template/config for the local provider test bed.
- [x] Automate k3s installation inside the Lima VM.
- [x] Install and validate containerd inside the Lima VM.
- [x] Install gVisor `runsc` inside the Lima VM.
- [x] Configure containerd with a `runsc` runtime handler.
- [x] Create and validate `RuntimeClass: gvisor`.
- [x] Install and configure MinIO.
- [x] Create local credentials/config for MinIO artifact storage.
- [x] Validate raw `runsc checkpoint` outside the Kubernetes CRD flow.
- [x] Validate raw `runsc restore` outside the Kubernetes CRD flow.
- [x] Upload checkpoint artifacts to MinIO.
- [x] Download checkpoint artifacts from MinIO for restore.
- [x] Add local cleanup for node-local checkpoint artifacts.
- [x] Add local provider error handling for missing runtime container IDs.
- [x] Add local provider error handling for failed artifact upload/download.
- [x] Add local provider status conditions for checkpoint, upload, restore, and cleanup.
- [x] Wire the Local Provider/Test Bed into the tagged E2E suite as a required provider path, not just a unit-level scaffold.
- [x] Make E2E fail clearly when the local test-bed prerequisites are missing.
- [x] Make E2E verify that the node agent handles `LocalRunscSnapshot` requests.
- [x] Make E2E verify raw runsc checkpoint/restore separately from the CRD flow.
- [x] Make E2E verify MinIO artifact storage separately from the CRD flow.
- [x] Make the Lima/k3s local cluster look and work like `stg-cluster` as much as practical.
- [x] Install the same `podsnapshot.gke.io/v1` CRDs in the Lima/k3s local cluster: `PodSnapshotStorageConfig`, `PodSnapshotPolicy`, `PodSnapshotManualTrigger`, and `PodSnapshot`.
- [x] Implement a local PodSnapshot shim controller that watches the GKE-shaped CRDs and fulfills them with runsc and MinIO.
- [x] Keep `LocalRunscSnapshot` as an internal node-agent request object, not the product-facing local E2E API.
- [x] Make local E2E drive snapshots through `SandboxSnapshot` and the GKE-shaped policy/manual-trigger flow, not by directly creating `LocalRunscSnapshot`.
- [x] Make the local provider mimic GKE policy/manual-trigger/readiness flow where practical.
- [x] Preserve GKE PodSnapshot as the source of truth for production behavior.
- [x] Document behavioral differences between GKE PodSnapshot and LocalRunsc.

## Product Acceptance

- [x] Add one API-only product acceptance test that exercises the full user workflow through the REST API, not direct Kubernetes CR writes.
- [x] API acceptance flow creates a sandbox through `POST /sandboxes`.
- [x] API acceptance flow waits for sandbox ready through `GET /sandboxes/{id}`.
- [x] API acceptance flow executes a command through the API/toolbox path.
- [x] API acceptance flow exposes a user port to the tailnet through the API.
- [x] API acceptance flow obtains SSH access through the API and connects through the CLI-facing SSH path.
- [x] API acceptance flow snapshots the sandbox through `POST /sandboxes/{id}:snapshot`.
- [x] API acceptance flow forks/restores through `POST /sandboxes/{id}:fork`.
- [x] API acceptance flow verifies restored toolbox access.
- [x] API acceptance flow verifies restored user port access.
- [x] API acceptance flow lists sandboxes through `GET /sandboxes`.
- [x] API acceptance flow stops the sandbox through `POST /sandboxes/{id}:stop`. ensure that resources are cleaned up.
- [x] API acceptance flow restarts the sandbox through an exec call - waking the box back up. 
- [x] stop the sandbox
- [x] API acceptance flow restarts the sandbox through `POST /sandboxes/{id}:start` ensure the sandbox is live again.
- [x] API acceptance flow destroys the sandbox through `DELETE /sandboxes/{id}`.
- [x] API acceptance flow verifies all owned Kubernetes resources and provider resources are cleaned up.

## Toolbox Sidecar

- [x] Implement real exec sessions against the workload container.
- [x] Implement port forwarding control used by the API/CLI tailnet exposure flow.
- [x] Implement SSH support for the CLI by having the API issue SSH connection details and route the connection to the sandbox toolbox over the tailnet-reachable control plane.
- [x] Add telemetry hooks for exec/files/ports/SSH operations.
- [x] Add credential reload after restore beyond env-derived identity.
- [x] Add readiness checks that verify identity, credentials, and routing state are refreshed after restore.
- [x] Add tests for closed external connections across checkpoint/restore.
- [x] Replace placeholder `501` responses for `/exec`, `/files`, `/ports`, and `/ssh`.

## GKE E2E

- [x] Use the current kubeconfig context `stg-cluster-operator.tail9212cd.ts.net` as the GKE E2E target.
- [x] Add an explicit E2E preflight that verifies the current context is exactly `stg-cluster-operator.tail9212cd.ts.net`.
- [x] Validate that GKE Pod Snapshot CRDs are installed on `stg-cluster`.
- [x] Validate that `RuntimeClass: gvisor` exists on `stg-cluster`.
- [x] Validate that a GKE Sandbox/gVisor-capable node pool exists on `stg-cluster`.
- [x] Validate that node machine types are compatible with Pod Snapshots.
- [x] Validate Workload Identity Federation configuration on `stg-cluster`.
- [x] Reference an existing stg Cloud Storage bucket from CI/Doppler configuration for `stg-cluster` E2E.
- [x] Create a temporary E2E-owned `PodSnapshotStorageConfig` that points at the configured stg bucket and a unique run ID prefix.
- [x] Create a sandbox on the GKE Sandbox node pool.
- [x] Create a manual GKE PodSnapshot through `SandboxSnapshot`.
- [x] Assert the manual snapshot reaches ready status.
- [x] Restore from the latest ready snapshot.
- [x] Restore from a specific snapshot.
- [x] Assert missing snapshots do not silently pass.
- [x] Assert not-ready snapshots do not silently pass.
- [x] Assert incompatible snapshot/spec combinations fail.
- [x] Assert toolbox access works after restore.
- [x] Assert PVC-backed state is rejected or clearly marked unsupported.
- [x] Clean up stg E2E sandboxes, snapshots, policies, triggers, and temporary storage config objects.
- [x] Do not provision or destroy disposable clusters as part of this E2E path.

## Stg-Cluster Safety

- [x] Run all stg E2E resources in a dedicated namespace such as `daytona-sandbox-e2e`.
- [x] Add a required E2E label to every test-created resource: `compute.daytona.io/e2e=true`.
- [x] Add a unique run ID label to every test-created resource.
- [x] Add cleanup that deletes only resources matching the E2E namespace and run ID.
- [x] Add preflight protection that refuses to run destructive cleanup outside the E2E namespace.
- [x] Add test timeouts for sandbox ready, snapshot ready, restore ready, stop, destroy, and cleanup.
- [x] Add a preflight that verifies the controller image under test is deployed before running destructive E2E.
- [x] Add a preflight that verifies `PodSnapshotStorageConfig` points at the expected stg bucket/path prefix.
- [x] Ensure tests never delete shared namespaces, shared buckets, shared RuntimeClasses, node pools, or cluster-scoped GKE PodSnapshot CRDs.
- [x] Ensure failed tests print enough resource names and commands to inspect stuck stg resources.

## Restore Semantics

- [x] Prove restored Pods boot correctly on GKE.
- [x] Prove restored Pods expose the expected toolbox endpoint.
- [x] Prove restored Pods expose configured user ports.
- [x] Add provider-specific restore status conditions.
- [x] Add explicit rejection for PVC-backed snapshotable sandboxes.
- [x] Add validation that GKE restore requests do not include unsupported volumes.
- [x] Strengthen compatibility checks beyond the current runtime spec hash.
- [x] Surface clear user-facing errors for provider restore failures.

## SandboxTemplate Integration

- [x] Make `SandboxSnapshot` reference the immutable `SandboxTemplate` used for compatibility.
- [x] Store template name and compatibility hash in snapshot status.
- [x] Require fork/restore to match the snapshot template.
- [x] Refuse restore when the requested sandbox spec does not match the template.
- [x] Make template compatibility the canonical restore contract instead of deriving only from the source `Sandbox`.
- [x] Add tests for template mismatch failures.
- [x] Add tests for template match success paths.

## Controller Rigor

- [x] Add controller-runtime `envtest` setup.
- [x] Test CRD schema validation with a real API server.
- [x] Test defaulting behavior with a real API server if defaulting is added.
- [x] Test Pod/Service/NetworkPolicy creation through envtest.
- [x] Test status updates through envtest.
- [x] Test deletion cleanup and finalizer behavior through envtest.
- [x] Test owned resource garbage collection assumptions.
- [x] Test failed reconcile and retry behavior.
- [x] Test GKE unstructured resource no-match behavior.
- [x] Test LocalRunsc request ownership and status propagation through envtest.

## Control API

- [x] Keep gRPC out of v1 unless a concrete consumer appears.
- [x] Treat tailnet access as the v1 authentication and authorization boundary.
- [x] Keep the API open to clients that can reach it on the tailnet.
- [x] Document that v1 is internal software and assumes tailnet access is trusted.
- [x] Add namespace scoping policy.
- [x] Add stronger request validation.
- [x] Add OpenAPI documentation.
- [x] Add generated API clients if needed.
- [x] Add API tests for access endpoint behavior once routing is real.
- [x] Add API tests for LocalRunsc stop/snapshot requests.
- [x] Add API tests for provider-specific restore errors.

## Access And Routing

- [x] Implement real tailnet-reachable sandbox access URLs returned by the API.
- [x] Route toolbox access through the control-plane API/proxy rather than exposing Pods directly.
- [x] Route user-declared ports through the control-plane API/proxy and expose them to the tailnet for CLI users.
- [x] Add per-sandbox routing records managed by the API/controller.
- [x] Rely on tailnet reachability as the v1 access boundary.
- [x] Add tests for access after start.
- [x] Add tests for access after restore.
- [x] Add tests for access after stop/destroy.

## Secrets And Configuration

- [x] Use Doppler as the v1 source for project secrets.
- [x] Make Doppler configuration selectable per project when creating a VM/test bed.
- [x] Add API fields or environment configuration for Doppler project/config selection.
- [x] Inject Doppler-provided secrets into sandbox workloads without storing secret values in Sandbox CR status.
- [x] Refresh Doppler-derived credentials after restore where needed.
- [x] Document required Doppler project/config setup for local Lima and stg-cluster E2E.

## Packaging And Deployment

- [x] Build and publish the multi-entrypoint image.
- [x] Add image tag/version strategy.
- [x] Add deployment overlays for GKE.
- [x] Add deployment overlays for the Lima-based local VM test bed.
- [x] Add RuntimeClass manifests where appropriate.
- [x] Add namespace creation for sandbox workload namespace.
- [x] Add configurable sandbox namespace in manifests.
- [x] Add NetworkPolicy for controller/API components if required.

## Garbage Collection

- [x] Add controller-owned labels and owner references to every sandbox-owned resource where Kubernetes ownership is valid.
- [x] Add explicit GC for resources that cannot use Kubernetes owner references.
- [x] Delete sandbox Pods, Services, NetworkPolicies, routing records, and access records when a `Sandbox` is destroyed.
- [x] Delete stop-created snapshots when they are owned by the sandbox and the sandbox is destroyed.
- [x] Do not delete user-created snapshots unless their owner/reference policy says they are sandbox-owned.
- [x] Delete GKE `PodSnapshotPolicy` and `PodSnapshotManualTrigger` resources owned by a `SandboxSnapshot`.
- [x] Delete temporary E2E `PodSnapshotStorageConfig` resources created by the test run.
- [x] Delete local PodSnapshot shim resources owned by a local test snapshot.
- [x] Delete `LocalRunscSnapshot` request objects owned by a `SandboxSnapshot`.
- [x] Clean up node-local runsc checkpoint artifacts created by failed or deleted local snapshots.
- [x] Clean up MinIO artifacts created by failed or deleted local snapshots in the Lima test bed.
- [x] Add stale-resource cleanup for snapshots stuck in pending/triggering beyond a configured timeout.
- [x] Add stale-resource cleanup for sandboxes stuck deleting beyond a configured timeout.
- [x] Add tests proving cleanup is scoped to owned resources and does not delete shared stg resources.

## Simple Observability

- [x] Emit Kubernetes events for sandbox create/start/stop/destroy.
- [x] Emit Kubernetes events for snapshot trigger/ready/failure.
- [x] Emit Kubernetes events for restore start/ready/failure.
- [x] Add structured controller logs with sandbox, snapshot, namespace, provider, and run ID fields.
- [x] Add simple Prometheus metrics for sandbox phases, snapshot phases, reconcile errors, and reconcile duration.
- [x] Add simple toolbox request logs for exec/files/ports/SSH operations.
- [x] Add simple node-agent logs for runsc checkpoint/restore and MinIO upload/download operations.

## Documentation

- [x] Document exact Lima Ubuntu VM setup.
- [x] Document Lima lifecycle commands for create/start/stop/delete.
- [x] Document exact k3s setup.
- [x] Document exact containerd `runsc` configuration.
- [x] Document exact MinIO setup.
- [x] Document raw runsc checkpoint/restore workflow.
- [x] Document LocalRunsc CRD flow.
- [x] Document `stg-cluster` E2E setup and prerequisites.
- [x] Document GKE PodSnapshot storage setup.
- [x] Document snapshot semantics and unsupported PVC state.
- [x] Document restore compatibility rules.
- [x] Document operational troubleshooting for stuck snapshots/restores.

## CI Gating

- [x] Configure PR CI to run unit tests.
- [x] Configure PR CI to run envtest tests.
- [x] Configure PR CI to run the Lima/k3s Local Provider/Test Bed E2E suite.
- [x] Configure PR CI to run the stg-cluster GKE E2E suite.
- [x] Configure PR CI to run the API-only product acceptance test.
- [x] Configure PR CI to fail on any leaked E2E resources after cleanup.
- [x] Configure PR CI to print cluster/resource diagnostics on failure.

## Test Plan Completion

- [x] Unit tests for CRD validation.
- [x] Unit tests for all restore compatibility edge cases.
- [x] Unit tests for finalizer edge cases.
- [x] Unit tests for all lifecycle state transitions.
- [x] Envtest tests for controller behavior.
- [x] Local cluster tests for sandbox create/start/stop/access on k3s inside the Lima VM.
- [x] Local runtime tests for raw `runsc checkpoint`.
- [x] Local runtime tests for raw `runsc restore`.
- [x] Local runtime tests for MinIO artifact storage.
- [x] E2E suite uses the Local Provider/Test Bed path from this plan.
- [x] E2E suite verifies local raw runsc checkpoint/restore independently from Kubernetes CRDs.
- [x] E2E suite verifies local MinIO artifact storage independently from Kubernetes CRDs.
- [x] E2E suite verifies LocalRunsc CRD flow through the node agent.
- [x] E2E suite targets `stg-cluster` for GKE coverage.
- [x] GKE E2E tests for snapshot ready status.
- [x] GKE E2E tests for latest snapshot restore.
- [x] GKE E2E tests for specific snapshot restore.
- [x] GKE E2E tests for missing/not-ready snapshot failures.
- [x] GKE E2E tests for toolbox access after restore.
- [x] GKE E2E tests for PVC-backed state rejection.
