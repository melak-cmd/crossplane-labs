# Kaonix Platform Configuration

A Crossplane v2 Configuration Package providing platform building blocks — deploy apps, networks, and PostgreSQL databases with a single YAML manifest.

## Features

| Composition | Resources Managed |
|---|---|
| `App` (`app-frontend`) | Deployment, HPA |
| `Network` (`network-fullstack`) | NetworkPolicy, Service, Ingress, ExternalName DNS |
| `Database` (`database-cnpg`) | CloudNativePG Cluster, Secret, optional ScheduledBackup |
| `DatabaseBackup` (`database-backup-cnpg`) | CloudNativePG Backup (on-demand volume snapshot) |

All XRDs use **namespaced scope** (`v2` API) so XRs live alongside your workloads.

## Prerequisites

- [k3d](https://k3d.io/) (or any existing Kubernetes cluster with Crossplane)
- [Helm 3](https://helm.sh/)
- [Crossplane CLI](https://docs.crossplane.io/latest/cli/) (v2.5+ with `project` support)
- [uptest](https://github.com/crossplane/uptest) + [chainsaw](https://github.com/kyverno/chainsaw) (for e2e tests)

## Quick Start

```bash
# 1. Create a k3d cluster (Traefik disabled)
make create-cluster

# 2. Install CSI snapshot support (snapshot CRDs, snapshot-controller, hostpath CSI driver, classes)
#    Must run before install-cnpg so CloudNativePG starts with volume snapshot support.
make install-csi

# 3. Install CloudNativePG operator
make install-cnpg

# 4. Install XRDs, Compositions, functions, providers and ProviderConfig
make install-deps

# 5. Deploy an app + network
kubectl apply -f examples/apps/app.yaml
kubectl apply -f examples/networks/network.yaml

# 6. Deploy a database
kubectl apply -f examples/databases/postgres.yaml
```

If the CloudNativePG operator was already running when `install-csi` added the snapshot CRDs, restart it instead of reinstalling:

```bash
make restart-cnpg   # delete cnpg operator pods; re-enables the volumeSnapshot backup method
```

`install-deps` applies the source manifests directly (`apis/`, `functions/`, `providers/`, `providers/providerconfigs/`). It does **not** install the `kaonix-platform` Configuration package, so Crossplane never auto-resolves the package's `dependsOn` functions/providers (they are pinned by the local manifests instead).

## Packaging & Build

The repo is a Crossplane **project** described by `crossplane-project.yaml` at the repo root (it replaces the legacy `crossplane.yaml` package manifest). It pins the same four dependencies as before — `function-go-templating >=v0.12.0`, `function-auto-ready >=v0.7.0`, `provider-kubernetes >=v1.3.0`, `provider-helm >=v1.3.0` — and the repository `registry.localhost:5000/kaonix-platform`.

```bash
make project-build   # crossplane project build → _output/kaonix-platform.xpkg (+ schemas/)
make project-push    # crossplane project push → push packages to the local registry
```

### Using `make setup` (full cluster bootstrap)

```bash
make setup          # create-cluster + install-crossplane + install-csi + install-cnpg + install-deps + wait for pods
```

## Example Resources

### App

```yaml
apiVersion: kaonix.com/v1alpha1
kind: App
metadata:
  name: my-app
  namespace: platform
spec:
  id: my-app
  crossplane:
    compositionSelector:
      matchLabels:
        type: frontend
  parameters:
    namespace: platform
    image: nginx:1.27-alpine
    port: 8080
```

### Network

```yaml
apiVersion: kaonix.com/v1alpha1
kind: Network
metadata:
  name: my-app-network
  namespace: platform
spec:
  id: my-app-network
  parameters:
    namespace: platform
    appName: my-app
    ports:
    - port: 8080
      protocol: TCP
    ingress:
      host: my-app.127.0.0.1.nip.io
      path: /
    networkPolicy:
      ingress:
      - 10.0.0.0/8
      egress:
      - 0.0.0.0/0
```

### Database

```yaml
apiVersion: kaonix.com/v1alpha1
kind: Database
metadata:
  name: my-db
  namespace: platform
spec:
  id: my-db
  parameters:
    namespace: platform
    database: myapp
    version: "16"
    size: small
    instances: 2
    storageSize: 2Gi
```

### Database backups

Backups use the CloudNativePG **volume snapshot** method (no object store or
credentials required). Scheduled backups are enabled per database:

```yaml
apiVersion: kaonix.com/v1alpha1
kind: Database
metadata:
  name: my-db
  namespace: platform
spec:
  id: my-db
  parameters:
    namespace: platform
    database: myapp
    version: "16"
    size: small
    instances: 1
    backup:
      schedule: "0 0 2 * * *"      # cron with seconds (daily at 02:00)
      # snapshotClass: csi-hostpath-snapclass  # optional, defaults to the cluster default
```

This configures the CNPG `Cluster` with `spec.backup.volumeSnapshot` and creates
a `ScheduledBackup` named `<id>-backup`. The `Database` status reports
`status.backup.schedule`, `status.backup.resourceName`, and
`status.backup.lastSuccessfulBackup` once CloudNativePG reports one.

On-demand backups use the `DatabaseBackup` XR:

```yaml
apiVersion: kaonix.com/v1alpha1
kind: DatabaseBackup
metadata:
  name: my-db-manual
  namespace: platform
spec:
  id: my-db   # id of the Database XR; the Backup targets Cluster my-db
```

This composes a CNPG `Backup` named `<id>-backup-manual` and mirrors its `phase`,
`startedAt`, `completionTime`, and `error` into the XR status. The name is
fixed, so to run another on-demand backup delete and re-apply the
`DatabaseBackup` XR (delete-recreate). The final status is pushed once the
composed `Backup` is re-observed by the provider; in this lab a fresh reconcile
of the XR (e.g. an annotation change) picks up `complete`/timestamps if the
provider observation lags.

> **Prerequisite:** volume snapshots require a CSI driver that supports
> snapshots plus a `VolumeSnapshotClass`. `make install-csi` sets this up on the
> lab cluster (external-snapshotter CRDs + snapshot-controller, the CSI
> hostpath driver + RBAC, the `csi-hostpath-sc` StorageClass, and the
> `csi-hostpath-snapclass` VolumeSnapshotClass). The default k3d/local-path
> storage does not support snapshots, so databases that need real backups must be
> provisioned on a snapshot-capable StorageClass via the `storageClass`
> parameter; otherwise the snapshot request is created but cannot complete.

#### Restore / recovery

Recovery is an operator-driven action (not automated) and targets a **new**
cluster identity. To recover a database from a volume-snapshot backup:

1. List the CNPG `Backup` objects and pick the one to restore from:

   ```bash
   kubectl get backups.postgresql.cnpg.io -n platform
   ```

2. Bootstrap a new CNPG `Cluster` from that backup using the recovery bootstrap
   (do this on the CNPG `Cluster` manifest, or by temporarily managing it
   outside the platform `Database` XR):

   ```yaml
   apiVersion: postgresql.cnpg.io/v1
   kind: Cluster
   metadata:
     name: my-db-restored
     namespace: platform
   spec:
     instances: 1
     storage:
       size: 1Gi
     bootstrap:
       recovery:
         method: volumeSnapshot
         backup:
           name: my-db-backup      # the Backup object to restore from
   ```

   For a point-in-time recovery, add `recoveryTarget` to the `recovery` stanza.
   See the CloudNativePG
   [Recovery](https://cloudnative-pg.io/docs/1.25/recovery) documentation for
   the full set of options.

3. Point the application at the restored cluster's service
   (`<name>-rw.<namespace>.svc`) and verify the data.

## Project Layout

```
crossplane-labs/
├── crossplane-project.yaml # Project definition (replaces crossplane.yaml)
├── apis/
│   ├── apps/               # App XRD (definition.yaml) + Composition (composition.yaml)
│   ├── databases/          # Database + DatabaseBackup XRDs + Compositions
│   └── networks/           # Network XRD (definition.yaml) + Composition (composition.yaml)
├── functions/
│   ├── functions.yaml      # go-templating, auto-ready, patch-and-transform, function-scale
│   └── function-scale/     # custom Go function (scale composed Deployments)
├── providers/
│   ├── kubernetes.yaml     # provider-kubernetes + DeploymentRuntimeConfig
│   ├── helm.yaml           # provider-helm + DeploymentRuntimeConfig
│   └── providerconfigs/    # default ProviderConfig (namespace: platform)
├── clusters/
│   └── k3d.yaml            # k3d cluster config
├── examples/
│   ├── apps/               # sample App XR
│   ├── databases/          # sample Database XR + backup examples
│   └── networks/           # sample Network XR
├── operations/             # placeholder for Operations manifests
├── tests/
│   └── uptest/
│       ├── setup.sh        # e2e setup (installs CRDs, providers, CNPG, webhook check)
│       └── app.yaml        # uptest manifest (5 resources, 300s timeout)
├── schemas/                # generated dependency schemas (from project build)
├── _output/                # generated packages (kaonix-platform.xpkg)
└── Makefile
```

## E2E Tests

Tests run via [uptest](https://github.com/crossplane/uptest) + [chainsaw](https://github.com/kyverno/chainsaw):

```bash
# Run full e2e suite (setup, apply, assert Ready, delete)
make uptest

# Inspect generated chainsaw test files without running them
make uptest-render
```

The setup script (`tests/uptest/setup.sh`) handles:
- Applying XRDs, Compositions, and functions
- Waiting for provider health and composition revisions
- Provider webhook endpoint validation
- CloudNativePG installation via Helm
- Platform namespace and ProviderConfig creation

## Local Render (requires Docker)

Preview composed resources without a cluster:

```bash
make render-app      # render App composition
make render-db       # render Database composition
make render-network  # render Network composition
```

Or directly:

```bash
crossplane render examples/apps/app.yaml apis/apps/composition.yaml \
  functions/functions.yaml -x
```

## Custom Functions

The repo hosts custom Crossplane composition functions (Go SDK) under `functions/`. The first one is **`function-scale`**: given an input like below, it sets `spec.replicas` on each desired composed resource whose `metadata.name` matches; unmatched targets surface a `Synced=False`/`TargetNotFound` condition and all other resources pass through unchanged.

```yaml
apiVersion: function-scale.fn.kaonix.com/v1beta1
kind: Input
spec:
  scaleTargets:
  - name: my-app
    replicas: 8
```

Create a new function from the official template:

```bash
crossplane xpkg init <function-name> function-template-go \
  -d functions/<function-name> -r
```

Then update the module path in `go.mod`, the `input/` types, and `fn.go`, and regenerate the input schema with `go generate ./...`.

### Function Make targets

```bash
make function-build    # go build -o function (Development runtime binary)
make function-test     # unit tests
make function-lint     # golangci-lint
make function-render   # local render of the example (requires Docker)
make function-xpkg     # Docker runtime image + .xpkg package
make function-push     # push image + .xpkg to the local registry
```

### Local development & render

```bash
make function-render
```

This builds the binary, runs it locally with `--insecure`, and renders
`functions/function-scale/example/` (the Function uses the `render.crossplane.io/runtime:
Development` annotation, so no cluster is needed). Manually:

```bash
cd functions/function-scale
go build -o function .
./function --insecure &   # listens on localhost:9443
crossplane render example/xr.yaml example/composition.yaml example/functions.yaml -x
```

### Deploying to a cluster

```bash
make function-push                  # build + push image and xpkg to registry.localhost:5000
```

The `Function` object for `function-scale` lives in `functions/functions.yaml`
(also applied by `install-deps`), so once the package is in the local registry
it is picked up automatically.

## Other Targets

```bash
make status          # cluster + Crossplane overview
make delete          # remove example XRs
make teardown        # delete XRs + destroy cluster
make project-build   # build project packages into _output/
make project-push    # build + push packages to the local registry
```

## CI

GitHub Actions runs e2e tests on every push (`.github/workflows/e2e.yaml`): creates a k3d cluster, installs Crossplane + providers, runs `make uptest`.
