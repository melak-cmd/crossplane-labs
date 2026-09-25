# function-recovery-python

A Python implementation of the `function-recovery` Crossplane Operation
Function. It supports `prepare`, `prepare-delete`, `delete`, `restore`, and
`cleanup` modes for CloudNativePG Clusters.

The package advertises the `operation` capability. Each Operation step must
provide the PostgreSQL XR as the required resource `postgresql`; restore also
requires the saved ConfigMap as `recovery-plan`. The Cluster name is resolved
from the PostgreSQL XR's `spec.crossplane.resourceRefs`, and the target
namespace is supplied in the input. Its input API is
`function-recovery-python.fn.kaonix.com/v1beta1`.

Prepare pauses the PostgreSQL XR and stores a sanitized Cluster manifest in a
ConfigMap. `prepare-delete` does both and waits for the Cluster to disappear.
Restore creates the Cluster from that plan with `spec.bootstrap.recovery`,
accepts an identical retry, and waits for `Ready=True`. Cleanup removes only the
recovery bootstrap. Kubernetes access uses the function Pod's in-cluster
service account; the existing `function-recovery-runtime` configuration and
RBAC in `operations/rbac.yaml` are reused.

## Test and Build

```shell
cd functions/function-recovery-python
hatch test
hatch fmt --check
docker build . --tag=registry.localhost:5000/function-recovery-python:v0.1.19
crossplane xpkg build \
   --package-root=package \
   --embed-runtime-image=registry.localhost:5000/function-recovery-python:v0.1.19 \
   --package-file=function-recovery-python.xpkg
crossplane xpkg push -f function-recovery-python.xpkg \
   registry.localhost:5000/function-recovery-python:v0.1.19
```

The Python Function was initialized with `crossplane xpkg init` using
Crossplane's `function-template-python`. After pushing the package, install it
with `kubectl apply -f functions/functions.yaml` from the repository root.

See [the Python Operation example](../../examples/databases/03-operation-recovery-python.yaml)
for the matching pipeline inputs and required resources.
