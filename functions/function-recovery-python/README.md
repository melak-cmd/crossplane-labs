# function-recovery-python

A Python implementation of the `function-recovery` Crossplane Operation
Function. It supports `prepare`, `delete`, `restore`, `cleanup`, and `resume`
modes for CloudNativePG Clusters.

The package advertises the `operation` capability. Each Operation step must
provide the PostgreSQL XR as the required resource `postgresql`. Restore reads
the saved ConfigMap named by `spec.planName` directly from the target namespace.
The Cluster name is resolved from the PostgreSQL XR's
`spec.crossplane.resourceRefs`, and the target namespace is supplied in the
input. Its input API is
`function-recovery-python.fn.kaonix.com/v1beta1`.

Prepare pauses the PostgreSQL XR and stores a sanitized Cluster manifest in a
ConfigMap. Restore creates the Cluster from that plan with `spec.bootstrap.recovery`,
accepts an identical retry, and waits for `Ready=True`. Cleanup removes only the
recovery bootstrap. Kubernetes access uses the function Pod's in-cluster
service account; the existing `function-recovery-runtime` configuration and
RBAC in `install/function-recovery-rbac.yaml` are reused.

For `DatabaseRestore` WatchOperations, the watched request provides the
PostgreSQL XR name, target namespace, and backup name. The Function acknowledges
the trigger label, treats the selector's synthetic deletion event as a no-op,
and can resume the request-derived PostgreSQL XR after cleanup.

## Test and Build

```shell
cd functions/function-recovery-python
hatch test
hatch fmt --check
docker build . --tag=registry.localhost:5000/function-recovery-python:v0.1.30
crossplane xpkg build \
   --package-root=package \
   --embed-runtime-image=registry.localhost:5000/function-recovery-python:v0.1.30 \
   --package-file=function-recovery-python.xpkg
crossplane xpkg push -f function-recovery-python.xpkg \
   registry.localhost:5000/function-recovery-python:v0.1.30
```

The Python Function was initialized with `crossplane xpkg init` using
Crossplane's `function-template-python`. After pushing the package, install it
with `kubectl apply -f install/functions.yaml` from the repository root.

See [the Python Operation example](../../examples/databases/03-operation-recovery-python.yaml)
for the matching pipeline inputs and required resources.

See [the DatabaseRestore recovery guide](../../docs/database-restore-recovery-python.md)
for the request-triggered workflow, file roles, and failure points.
