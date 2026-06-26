#!/usr/bin/env bash
set -euo pipefail

VM_NAME="${1:-daytona-sandbox}"

limactl shell --workdir / "$VM_NAME" bash -lc '
set -euo pipefail
sudo k3s kubectl -n daytona-system run minio-smoke --rm -i --restart=Never \
  --image=quay.io/minio/mc:latest \
  --env=MC_HOST_local=http://daytona:daytona-local-password@local-minio.daytona-system.svc.cluster.local:9000 \
  --command -- sh -lc "
    set -euo pipefail
    echo ok >/tmp/daytona-minio-smoke
    mc mb --ignore-existing local/daytona-local
    mc cp /tmp/daytona-minio-smoke local/daytona-local/snapshots/raw/daytona-minio-smoke
    mc cp local/daytona-local/snapshots/raw/daytona-minio-smoke /tmp/daytona-minio-smoke.out
    test \"\$(cat /tmp/daytona-minio-smoke.out)\" = ok
  "
'
