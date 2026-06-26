# Troubleshooting Snapshots And Restores

## Local Lima

```bash
apps/sandbox-controller/hack/local-provider/preflight.sh daytona-sandbox
limactl shell daytona-sandbox sudo k3s kubectl -n daytona-system logs deploy/sandbox-controller
limactl shell daytona-sandbox sudo k3s kubectl -n daytona-system logs ds/local-node-agent
limactl shell daytona-sandbox sudo k3s kubectl -n sandboxes get sandboxsnapshots,podsnapshots,podsnapshotmanualtriggers,localrunscsnapshots
```

Common causes:

- `RuntimeClass gvisor` missing: rerun provisioning and inspect containerd config.
- `LocalRunscSnapshot` stuck pending: source Pod is not scheduled or the workload container has no runtime container ID yet.
- Upload/download failure: check `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`, bucket, endpoint, and `local-minio` readiness.

## stg-cluster

```bash
kubectl config current-context
kubectl get crd podsnapshots.podsnapshot.gke.io podsnapshotpolicies.podsnapshot.gke.io podsnapshotmanualtriggers.podsnapshot.gke.io podsnapshotstorageconfigs.podsnapshot.gke.io
kubectl get runtimeclass gvisor
kubectl get nodes -L cloud.google.com/gke-sandbox,cloud.google.com/gke-nodepool,node.kubernetes.io/instance-type
kubectl -n daytona-sandbox-e2e get sandboxes,sandboxsnapshots,podsnapshots,podsnapshotmanualtriggers -l compute.daytona.io/e2e=true
```

Failed E2E output includes run IDs and resource names. Use the run ID label to inspect or clean up only test-owned resources.
