#!/usr/bin/env bash
set -euo pipefail

export DEBIAN_FRONTEND=noninteractive

apt-get update
apt-get install -y --no-install-recommends \
  apt-transport-https \
  ca-certificates \
  conntrack \
  curl \
  gnupg \
  jq \
  make \
  socat \
  uidmap \
  wget

install -d -m 0755 /usr/share/keyrings
if [ ! -f /usr/share/keyrings/gvisor-archive-keyring.gpg ]; then
  curl -fsSL https://gvisor.dev/archive.key | gpg --dearmor -o /usr/share/keyrings/gvisor-archive-keyring.gpg
fi
cat >/etc/apt/sources.list.d/gvisor.list <<EOF
deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/gvisor-archive-keyring.gpg] https://storage.googleapis.com/gvisor/releases release main
EOF
apt-get update
apt-get install -y --no-install-recommends runsc

if ! command -v k3s >/dev/null 2>&1; then
  curl -sfL https://get.k3s.io | INSTALL_K3S_EXEC="--write-kubeconfig-mode=0644 --disable=traefik" sh -
fi

install -d -m 0755 /var/lib/rancher/k3s/agent/etc/containerd
cat >/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl <<'EOF'
{{ template "base" . }}

[plugins."io.containerd.grpc.v1.cri".containerd.runtimes.gvisor]
  runtime_type = "io.containerd.runsc.v1"
EOF
systemctl restart k3s

until k3s kubectl get nodes >/dev/null 2>&1; do
  sleep 2
done

k3s kubectl apply -k /workspace/daytona/apps/sandbox-controller/config
k3s kubectl apply -k /workspace/daytona/apps/sandbox-controller/config/local
k3s kubectl wait --for=condition=Established crd/podsnapshotstorageconfigs.podsnapshot.gke.io --timeout=120s
k3s kubectl apply -f /workspace/daytona/apps/sandbox-controller/config/local/podsnapshot_storageconfig.yaml
k3s kubectl -n daytona-system patch deployment sandbox-controller --type=strategic --patch-file /workspace/daytona/apps/sandbox-controller/config/local/manager_local_shim_patch.yaml
k3s kubectl -n daytona-system rollout status deploy/local-minio --timeout=180s
k3s kubectl -n daytona-system rollout status deploy/sandbox-controller --timeout=180s || true
k3s kubectl -n daytona-system rollout status deploy/sandbox-api --timeout=180s || true

runsc --version
k3s ctr version
k3s kubectl get runtimeclass gvisor
