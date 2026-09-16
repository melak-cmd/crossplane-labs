# Write a Custom Crossplane Function (Go SDK)

## Why

The platform currently trusts only upstream composition functions (`function-go-templating`, `function-patch-and-transform`, `function-auto-ready`). There is no in-house authoring pattern for extending the composition pipeline with custom logic. This change establishes that pattern by scaffolding, building, and deploying a minimal Go SDK function with `crossplane function init`, proving the authoring → build → render → deploy loop end to end before any real pipeline logic is written.

## What Changes

- Add a standalone Go function package `functions/function-scale/`, scaffolded with `crossplane function init` (Crossplane CLI). It implements the reference SDK flow — `RunFunction` reads its `Input`, which maps app names to replica counts, and for each desired composed resource whose name matches an entry, sets `spec.replicas` to the given value; all other desired resources pass through untouched. Doubles as a learning/pattern PoC for the authoring loop (init → implement → build → render → deploy).
- Add Makefile targets for the function lifecycle: `function-lint`, `function-test`, `function-build`, `function-push` (mirroring the existing `project-*` pattern and pinning a registry `registry.localhost:5000/function-scale`).
- Add a Function deployment manifest in `functions/functions.yaml` (new `pkg.crossplane.io/v1` `Function` for `function-scale`), installed by the existing `install-deps` flow — alongside, not replacing, the upstream functions.
- Add a `crossplane render` dry-run/test example exercising the scale behavior (XR with an `Input` mapping app names to replica counts against a Deployment composition); the function is **not** wired into any real composition yet.
- Update README with a "Custom Functions" section documenting the authoring pattern (init → implement → test → build → push → deploy).

## Capabilities

### New Capabilities

- `custom-functions`: In-house Crossplane composition functions authored in Go with the Crossplane CLI SDK. Covers scaffolding via `crossplane function init`, the `RunFunction` input/desired-resource pattern, package build/push, deployment as a `Function` in the platform, and cluster-less `crossplane render` validation. First function is `function-scale`, which scales composed Deployments by app name from its `Input`.

### Modified Capabilities

None — `openspec/specs/` currently holds no capability that changes behavior; the new function is additive and no existing composition or XRD requirement changes.

## Impact

- **New module**: `functions/function-scale/` — Go module scaffolded by `crossplane function init`, built on `crossplane-function-sdk-go`, pinned in `go.mod`/`go.sum`.
- **Manifests**: one new `Function` entry in `functions/functions.yaml` (package `registry.localhost:5000/function-scale`); no changes to existing function pins, XRDs, compositions, or providers.
- **Makefile**: new `function-*` targets; existing targets untouched.
- **Docs**: README gains a "Custom Functions" section and layout entry.
- **Tooling**: requires the Crossplane CLI (with `crossplane function init`) and a Go toolchain; Docker needed for `xpkg build`/push (same requirement as the existing project build).
- **Dependencies**: adds `crossplane-function-sdk-go` at the `go.mod` level. No runtime dependency beyond the SDK and Go runtime.