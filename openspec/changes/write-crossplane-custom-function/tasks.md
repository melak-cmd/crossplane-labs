## 1. Scaffold the function package

- [x] 1.1 Scaffold `functions/function-scale/` with `crossplane xpkg init function-scale function-template-go -d functions/function-scale -r` and verify the template files are present (fn.go, main.go, input/v1beta1, package/crossplane.yaml, example/, Dockerfile, .golangci.yml, .github/workflows/ci.yml)
- [x] 1.2 Personalize the module: replace `function-template-go`/`fnscale` with `function-scale` in `go.mod` (module `github.com/melak-cmd/crossplane-labs/functions/function-scale`), `fn.go` import, and set `package/crossplane.yaml` `metadata.name: function-scale`; verify `go mod tidy` and `go build ./...` succeed
- [x] 1.3 Keep the template's `Dockerfile`, `.golangci.yml`, CI workflow and remove the template's throwaway `LICENSE`/`NOTES.txt` if they reference the upstream project; verify the function directory contains no stale `function-template-go` references

## 2. Define the Input type

- [x] 2.1 Add `Spec` with `ScaleTargets []ScaleTarget` (`Name string` `json:"name"`, `Replicas int32` `json:"replicas"`) to `input/v1beta1/input.go` under group `function-scale.fn.kaonix.com`, keep `kind: Input`; keep `+kubebuilder` markers
- [x] 2.2 Run `go generate ./...` to regenerate `zz_generated.deepcopy.go` and the input CRD schema in `package/input/`; verify `git status` shows the regenerated `package/input/function-scale.fn.kaonix.com_inputs.yaml` and deepcopy updates

## 3. Implement RunFunction

- [x] 3.1 Implement `RunFunction` in `fn.go`: decode input via `request.GetInput`; empty/missing input or empty `scaleTargets` passes desired state through unchanged with no added conditions
- [x] 3.2 Add the scale matching loop: for each `scaleTargets` entry, set `spec.replicas` on the desired composed resource whose `metadata.name` matches, using `request.GetDesiredComposedResources`/`response.SetDesiredComposedResources`; enumerate and set `Ready=True`/`Synced=True` (reason `Succeeded`) and respond with `response.DefaultTTL`
- [x] 3.3 Handle unmatched targets: when no desired resource matches an input name, record a `Synced=False` condition with reason `TargetNotFound` (naming the target) while still passing resources through; keep fatal results only for internal decode/resource errors
- [x] 3.4 Update the input schema example in `package/input/*.yaml` comment/docs if the generator leaves stale example fields; verify `go vet ./...` and `go build ./...` pass

## 4. Unit tests

- [x] 4.1 Rework `fn_test.go` into a table of spec scenarios: valid target patches the matching Deployment's `spec.replicas`; unmatched target returns resources unchanged plus `TargetNotFound` condition; empty input returns resources unchanged without added conditions; multiple targets patch independently; non-matching resources are passed through byte-for-byte; verify `go test ./...` passes all cases

## 5. Renderable example (dev runtime)

- [x] 5.1 Update `example/` for `function-scale`: `xr.yaml` (sample XR), `composition.yaml` (pipeline with `functionRef: function-scale` and an `input` containing `scaleTargets` for an app Deployment), and `functions.yaml` with the `render.crossplane.io/runtime: Development` annotation
- [x] 5.2 Verify the example renders locally: `go build -o function . && crossplane render example/xr.yaml example/composition.yaml example/functions.yaml -x` shows the matched Deployment with `spec.replicas` set as requested and unmatched resources unchanged

## 6. Platform integration

- [x] 6.1 Add `Function` manifest entry `function-scale` (`spec.package: registry.localhost:5000/function-scale`) to `functions/functions.yaml`; verify `kubectl`-applied YAML parses (no cluster required: `kubectl apply --dry-run=client -f functions/functions.yaml -o yaml` succeeds)
- [x] 6.2 Add `function-build`, `function-test`, `function-lint`, `function-xpkg`, `function-push`, `function-render` targets to the repo `Makefile`; verify `make function-build` and `make function-test` succeed
- [x] 6.3 Add a "Custom Functions" section to `README.md` (authoring loop, prerequisites including Go toolchain note, targets); verify README renders without broken references

## 7. Validation

- [x] 7.1 Run `openspec validate --change write-crossplane-custom-function --strict` (or nearest-equivalent) and verify the change validates with no errors