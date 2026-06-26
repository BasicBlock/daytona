#!/usr/bin/env bash
set -euo pipefail

VM_NAME="${1:-daytona-sandbox}"

limactl shell --workdir / "$VM_NAME" sudo bash -lc '
set -euo pipefail
rm -rf /var/lib/daytona-localrunsc/*
k3s kubectl -n sandboxes delete sandboxes.compute.daytona.io --all --ignore-not-found
k3s kubectl -n sandboxes delete sandboxsnapshots.compute.daytona.io --all --ignore-not-found
k3s kubectl -n sandboxes delete localrunscsnapshots.compute.daytona.io --all --ignore-not-found
k3s kubectl -n sandboxes delete podsnapshotpolicies.podsnapshot.gke.io --all --ignore-not-found
k3s kubectl -n sandboxes delete podsnapshotmanualtriggers.podsnapshot.gke.io --all --ignore-not-found
k3s kubectl -n sandboxes delete podsnapshots.podsnapshot.gke.io --all --ignore-not-found
'
