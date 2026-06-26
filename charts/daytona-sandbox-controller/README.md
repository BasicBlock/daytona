# daytona-sandbox-controller

Helm chart for the Daytona sandbox controller, sandbox API, CRDs, RBAC, optional GKE PodSnapshot storage config, and optional Tailscale Ingress.

The chart is published as an OCI Helm artifact to:

```bash
oci://us-central1-docker.pkg.dev/basicblock/helm-charts/daytona-sandbox-controller
```

The repo `Release` workflow publishes this chart using the release version without the leading `v`; `.github/workflows/sandbox-controller-helm.yml` also supports PR linting and manual chart publication.

## Package and publish

```bash
CHART_VERSION=0.1.0
APP_VERSION=$(git rev-parse --short HEAD)

helm package charts/daytona-sandbox-controller \
  --version "$CHART_VERSION" \
  --app-version "$APP_VERSION" \
  --destination /tmp/charts

helm push "/tmp/charts/daytona-sandbox-controller-$CHART_VERSION.tgz" \
  oci://us-central1-docker.pkg.dev/basicblock/helm-charts
```

## Install example

```bash
helm upgrade --install daytona-sandbox-controller \
  oci://us-central1-docker.pkg.dev/basicblock/helm-charts/daytona-sandbox-controller \
  --version "$CHART_VERSION" \
  --namespace daytona-system \
  --create-namespace \
  --set image.tag="$IMAGE_TAG" \
  --set api.publicBaseUrl="https://basicblock-daytona-stg.tail9212cd.ts.net" \
  --set api.ingress.enabled=true \
  --set api.ingress.hostname=basicblock-daytona-stg \
  --set gke.podSnapshotStorageConfig.enabled=true \
  --set gke.podSnapshotStorageConfig.bucket=basicblock-trigger-snapshots \
  --set gke.podSnapshotStorageConfig.path=sandbox-controller
```

CRDs live under `templates/crds` so normal Helm rendering and upgrades include CRD schema changes.

For stg adoption of resources that were previously applied with `kubectl`, add:

```bash
--take-ownership \
--server-side=false \
--set namespaces.create=false
```
