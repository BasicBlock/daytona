# Packaging And Deployment

The Dockerfile builds one image with four entrypoints:

- `/sandbox-controller`
- `/sandbox-api`
- `/local-node-agent`
- `/toolbox-sidecar`

Use immutable tags for environments:

```text
ghcr.io/daytonaio/sandbox-controller:<git-sha>
ghcr.io/daytonaio/sandbox-controller:pr-<number>-<git-sha>
ghcr.io/daytonaio/sandbox-controller:stg-<date>-<git-sha>
```

The mutable `:dev` tag is only for local Lima work.

## Build

```bash
docker build -f apps/sandbox-controller/Dockerfile -t ghcr.io/daytonaio/sandbox-controller:$(git rev-parse --short HEAD) .
```

## Base Install

```bash
kubectl apply -k apps/sandbox-controller/config
```

## GKE Add-Ons

```bash
kubectl apply -k apps/sandbox-controller/config/gke
```

The GKE add-on creates the sandbox workload namespace and a minimal NetworkPolicy for the control-plane components.

## Lima Add-Ons

```bash
kubectl apply -k apps/sandbox-controller/config/local
kubectl wait --for=condition=Established crd/podsnapshotstorageconfigs.podsnapshot.gke.io --timeout=120s
kubectl apply -f apps/sandbox-controller/config/local/podsnapshot_storageconfig.yaml
kubectl -n daytona-system patch deployment sandbox-controller --type=strategic --patch-file apps/sandbox-controller/config/local/manager_local_shim_patch.yaml
```

The Lima provisioner applies the base stack, local add-ons, and the shim patch automatically. Then run:

```bash
apps/sandbox-controller/hack/local-provider/load-dev-image.sh daytona-sandbox
```
