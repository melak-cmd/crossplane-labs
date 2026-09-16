# function-scale

A Crossplane [composition function][functions] written in [Go][go] that sets
`spec.replicas` on desired composed Deployments by app name.

Given an `Input` (`apiVersion: function-scale.fn.kaonix.com/v1beta1`, kind
`Input`) with a `spec.scaleTargets` list of `{name, replicas}` entries, the
function patches the desired composed resource whose `metadata.name` matches
each entry, leaving all other resources untouched.

It is built with [Go][go], [Docker][docker], and the [Crossplane CLI][cli].

```shell
# Run code generation - see input/generate.go
$ go generate ./...

# Run tests - see fn_test.go
$ go test ./...

# Build the function's runtime image - see Dockerfile
$ docker build . --tag=runtime

# Build a function package - see package/crossplane.yaml
$ crossplane xpkg build -f package --embed-runtime-image=runtime
```

See `example/` for a cluster-less `crossplane render` demo using the
Development runtime, and the platform `Makefile` `function-*` targets.

[functions]: https://docs.crossplane.io/latest/concepts/composition-functions
[go]: https://go.dev
[docker]: https://www.docker.com
[cli]: https://docs.crossplane.io/latest/cli