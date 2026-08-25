# Kaonix Platform Configuration

A Crossplane v2 Configuration Package providing platform building blocks — deploy apps, networks, and PostgreSQL databases with a single YAML manifest.

## Features

| Composition | Resources Managed |
|---|---|
| `App` (`app-frontend`) | Deployment, HPA |
| `Network` (`network-fullstack`) | NetworkPolicy, Service, Ingress, ExternalName DNS |
| `Database` (`database-cnpg`) | CloudNativePG Cluster, Secret |

All XRDs use **namespaced scope** (`v2` API) so XRs live alongside your workloads.

## Prerequisites

- [k3d](https://k3d.io/) (or any existing Kubernetes cluster with Crossplane)
- [Helm 3](https://helm.sh/)
- [Crossplane CLI](https://docs.crossplane.io/latest/cli/) (`crossplane` / `xpkg`)
- [uptest](https://github.com/crossplane/uptest) + [chainsaw](https://github.com/kyverno/chainsaw) (for e2e tests)

## Quick Start

```bash
# 1. Create a k3d cluster (Traefik disabled)
make create-cluster

# 2. Install CloudNativePG operator
make install-cnpg

# 3. Deploy XRDs, Compositions, functions and provider config
make deploy

# 4. Deploy an app + network
kubectl apply -f examples/apps/app.yaml
kubectl apply -f examples/networks/network.yaml

# 5. Deploy a database
kubectl apply -f examples/databases/postgres.yaml
```

### Using `make setup` (full cluster bootstrap)

```bash
make setup          # create-cluster + install-cnpg + deploy + wait for pods
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

## Project Layout

```
crossplane-labs/
├── apis/
│   ├── apps/               # App XRD + Composition (Deployment, HPA)
│   ├── databases/          # Database XRD + Composition (CNPG Cluster)
│   └── networks/           # Network XRD + Composition (NetworkPolicy, Service, Ingress, DNS)
├── crossplane/
│   ├── functions/          # go-templating, auto-ready, patch-and-transform
│   ├── providers/          # kubernetes, helm providers + DeploymentRuntimeConfig
│   └── providerconfigs/    # default ProviderConfig (namespace: platform)
├── clusters/
│   └── k3d.yaml            # k3d cluster config
├── examples/
│   ├── apps/               # sample App XR
│   ├── databases/          # sample Database XR
│   └── networks/           # sample Network XR
├── test/
│   └── uptest/
│       ├── setup.sh        # e2e setup (installs CRDs, providers, CNPG, webhook check)
│       └── app.yaml        # uptest manifest (5 resources, 300s timeout)
├── crossplane.yaml         # Configuration package metadata
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

The setup script (`test/uptest/setup.sh`) handles:
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
  crossplane/functions/functions.yaml -x
```

## Other Targets

```bash
make status          # cluster + Crossplane overview
make delete          # remove example XRs
make teardown        # delete XRs + destroy cluster
make build-xpkg      # build crossplane.xpkg package
```

## CI

GitHub Actions runs e2e tests on every push (`.github/workflows/e2e.yaml`): creates a k3d cluster, installs Crossplane + providers, runs `make uptest`.
