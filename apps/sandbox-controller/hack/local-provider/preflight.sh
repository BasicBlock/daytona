#!/usr/bin/env bash
set -euo pipefail

VM_NAME="${1:-daytona-sandbox}"

limactl shell --workdir / "$VM_NAME" bash -lc '
set -euo pipefail
command -v runsc
sudo runsc --version
sudo k3s ctr version
sudo k3s kubectl get runtimeclass gvisor
sudo k3s kubectl get crd \
  podsnapshotstorageconfigs.podsnapshot.gke.io \
  podsnapshotpolicies.podsnapshot.gke.io \
  podsnapshotmanualtriggers.podsnapshot.gke.io \
  podsnapshots.podsnapshot.gke.io
sudo k3s kubectl -n daytona-system get deploy sandbox-controller sandbox-api local-minio
sudo k3s kubectl -n daytona-system get daemonset local-node-agent
sudo k3s kubectl -n daytona-system rollout status deploy/sandbox-controller --timeout=120s
sudo k3s kubectl -n daytona-system rollout status deploy/sandbox-api --timeout=120s
sudo k3s kubectl -n daytona-system rollout status deploy/local-minio --timeout=120s
sudo k3s kubectl -n daytona-system rollout status ds/local-node-agent --timeout=120s
'
