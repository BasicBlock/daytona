# Doppler Configuration

Doppler is the v1 source for project secrets. Sandbox specs select a Doppler project/config and the Kubernetes Secret name populated by the secret-sync path:

```yaml
spec:
  secrets:
    provider: doppler
    dopplerProject: daytona
    dopplerConfig: stg
    managedSecretName: sandbox-agent-doppler
```

The controller does not write secret values into `Sandbox.status`. Rendered workload containers consume the managed Secret through `envFrom`. The toolbox sidecar receives only metadata (`DAYTONA_DOPPLER_PROJECT`, `DAYTONA_DOPPLER_CONFIG`, and `DAYTONA_CREDENTIAL_VERSION`) so readiness checks reload identity/credential routing state after restore without exposing secret material.

For local Lima tests, create the managed Secret in `sandboxes` or install the same Doppler secret operator used by staging. For stg-cluster E2E, CI/Doppler supplies bucket/API configuration and the cluster secret-sync path supplies workload secrets.
