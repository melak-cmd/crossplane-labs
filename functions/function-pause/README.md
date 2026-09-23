# function-pause

A Crossplane composition [Function][functions] that pauses or resumes a single
referenced Crossplane resource. It is designed to run in an [Operation][operations]
pipeline, not in a Composition, so it never modifies database or backup
Compositions.

Given an `Input` (`apiVersion: function-pause.fn.kaonix.com/v1beta1`, kind
`Input`) with `spec.target` (`apiVersion`, `kind`, `name`, optional
`namespace`) and `spec.paused`, the function reads the resource the Operation
preloaded as a required resource named `target`, and returns it with
`metadata.annotations["crossplane.io/paused"]` set to `"true"` when
`paused: true`, or removed when `paused: false`. All other resource fields are
left untouched.

## Using it in an Operation

```yaml
apiVersion: ops.crossplane.io/v1alpha1
kind: Operation
metadata:
  name: pause-orders-database
spec:
  mode: Pipeline
  pipeline:
  - step: pause
    functionRef:
      name: function-pause
    requirements:
      requiredResources:
      - requirementName: target
        apiVersion: kaonix.com/v1alpha1
        kind: Database
        name: orders
        namespace: platform
    input:
      apiVersion: function-pause.fn.kaonix.com/v1beta1
      kind: Input
      spec:
        target:
          apiVersion: kaonix.com/v1alpha1
          kind: Database
          name: orders
          namespace: platform
        paused: true
```

To resume, apply a second Operation with `paused: false`. See
`example/pause-operation.yaml` and `example/resume-operation.yaml` for
complete manifests, and `example/target.yaml` for a sample required resource
used with `crossplane operation render`.

This pauses Crossplane's reconciliation of the target resource. It does not
stop the underlying provider-managed workload (for example, a running
CloudNativePG cluster), and it does not suspend CNPG `ScheduledBackup`
resources.

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=runtime

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=runtime

# Render the pause example locally (requires Docker and Crossplane alpha features)
$ ./function --insecure &
$ crossplane operation render example/pause-operation.yaml example/functions.yaml \
    --required-resources=example/target.yaml
```

[functions]: https://docs.crossplane.io/latest/concepts/composition-functions
[operations]: https://docs.crossplane.io/latest/operations/operation/

