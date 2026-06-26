#!/usr/bin/env bash
set -euo pipefail

VM_NAME="${1:-daytona-sandbox}"
IMAGE="${2:-ghcr.io/daytonaio/sandbox-controller:dev}"
TAR_PATH="/tmp/sandbox-controller-dev.tar"

docker build -f apps/sandbox-controller/Dockerfile -t "$IMAGE" .
docker save "$IMAGE" -o "$TAR_PATH"
limactl copy "$TAR_PATH" "$VM_NAME:$TAR_PATH"
limactl shell --workdir / "$VM_NAME" sudo k3s ctr images import "$TAR_PATH"
limactl shell --workdir / "$VM_NAME" sudo k3s kubectl -n daytona-system rollout restart deploy/sandbox-controller deploy/sandbox-api ds/local-node-agent
limactl shell --workdir / "$VM_NAME" sudo k3s kubectl -n daytona-system rollout status deploy/sandbox-controller --timeout=180s
limactl shell --workdir / "$VM_NAME" sudo k3s kubectl -n daytona-system rollout status deploy/sandbox-api --timeout=180s
limactl shell --workdir / "$VM_NAME" sudo k3s kubectl -n daytona-system rollout status ds/local-node-agent --timeout=180s
