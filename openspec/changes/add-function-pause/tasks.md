## 1. Function Package

- [x] 1.1 Scaffold `functions/function-pause/` from the existing Go Crossplane function pattern and verify the expected source, module, package, and example files are present.
- [x] 1.2 Define the typed `function-pause.fn.kaonix.com/v1beta1` Input with one `target` reference and boolean `paused`, regenerate deepcopy and input CRD output, and verify the generated schema requires the target identity fields.
- [x] 1.3 Implement Operation target resolution and annotation application through the selected resource-application mechanism; verify pause and resume never alter the target operational spec or database Composition.
- [x] 1.4 Report invalid or missing targets with deterministic conditions such as `TargetNotFound` while retaining unrelated resources; verify failure behavior in unit tests.

## 2. Packaging And Repository Integration

- [x] 2.1 Add function package metadata and a local installation entry for `function-pause`, using the repository's existing function naming and registry conventions; verify package manifests parse and the function is included by `kubectl apply -f functions/`.
- [x] 2.2 Extend the Makefile function lifecycle so `FUNCTION_NAME=function-pause` supports build, test, lint, xpkg, push, and render commands; verify the targets resolve the new function directory and image name without changing the default `function-scale` behavior.
- [x] 2.3 Add an Operation example that invokes `function-pause` for a named XR and a separate resume example; verify the manifests use the Operation API supported by the pinned Crossplane version.

## 3. Validation And Documentation

- [x] 3.1 Add README or package usage documentation showing how to invoke pause and resume Operations for a Database or DatabaseBackup XR and clarify that the function pauses Crossplane management rather than application execution; verify the documented input matches the generated schema.
- [ ] 3.2 Run `go test ./...`, `go build ./...`, and the function lint command in `functions/function-pause/`; verify all pass.
- [x] 3.3 Run the Operation/function render example and repository validation relevant to the updated function manifests; verify existing database compositions remain renderable without modification and the new example demonstrates reversible pause behavior.