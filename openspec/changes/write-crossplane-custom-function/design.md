# Design: function-scale (custom Crossplane function, Go SDK)

## Context

See `proposal.md` — Why. This change adds the platform's first in-house composition function, `function-scale`, as a standalone package under `functions/function-scale/`.

Current state and constraints:

- The platform is a Crossplane v2 project (`crossplane-project.yaml`, CLI v2.5.0, Crossplane target v2.3.3). The repo's three pipeline Compositions use upstream functions (`function-go-templating`, `function-patch-and-transform`, `function-auto-ready`) declared in `functions/functions.yaml` and installed directly by `make install-deps`.
- The Crossplane CLI v2.x **no longer ships `crossplane function init`**. The current CLI-native scaffolding path is `crossplane xpkg init <name> function-template-go -d <dir>`, which clones `github.com/crossplane/function-template-go`. Verified on this machine (CLI v2.5.0): `crossplane xpkg init fnscale function-template-go -d /tmp/opencode/fnscale -r` produces a working module.
- Template reality (verified by scaffolding a throwaway): it targets `go 1.25.10` while the machine has `go 1.22.2`; `GOTOOLCHAIN=auto` downloads go1.25.10 and `go build`/`go test` pass. SDK is `function-sdk-go v0.7.1` (Crossplane v2 compatible).
- Template layout: `main.go` (kong CLI → `function.Serve`), `fn.go` (RunFunction), `fn_test.go`, `input/v1beta1/{input.go,zz_generated.deepcopy.go}`, `input/generate.go` (controller-gen → CRD schema in `package/input/`), `package/crossplane.yaml` (package metadata), `example/{xr,composition,functions}.yaml`, `Dockerfile`, `.golangci.yml`, `.github/workflows/ci.yml`, `init.sh`.
- Template `init.sh` replaces `function-template-go` in `go.mod`, `fn.go`, and `example/*` **only** — `package/crossplane.yaml` and the input `groupName` must be renamed by hand.
- SDK behavior (confirmed in `function-sdk-go v0.7.1`): `response.To(req, response.DefaultTTL)` copies `desired`, `context`, and `tag` into the response — untouched desired resources pass through automatically. Patching is done via `request.GetDesiredComposedResources(req)` → mutate matching entries → `response.SetDesiredComposedResources(rsp, dcds)`. `request.GetInput(req, &v1beta1.Input{})` decodes the function `input`.

Spec contract: see `specs/custom-functions/spec.md` (input `function-scale.fn.kaonix.com/v1beta1`, `spec.scaleTargets`, Deployment `spec.replicas` patching, passthrough, `Ready`/`Synced` conditions, `TargetNotFound`, TTL).

## Goals / Non-Goals

**Goals:**
- Establish a repeatable in-house Go-function authoring loop: scaffold → define input → implement → test → build → render → deploy.
- Ship `function-scale`: reads `scaleTargets` (app name → replicas) and sets `spec.replicas` on matching desired Decompositions, passing everything else through.
- Unit-tested behavior that mirrors the spec scenarios, and a cluster-less `crossplane render` demo using the dev runtime.

**Non-Goals:**
- Wiring `function-scale` into any real Composition/XR (the App/Network/Database platform capabilities are untouched).
- HPA awareness, autoscaling coordination, or reading scale intent from XRs — the function is input-driven only.
- Publishing `function-scale` to a public registry; push targets the local `registry.localhost:5000` like the rest of the repo.

## Decisions

**D1 — Scaffold source: `crossplane xpkg init function-scale function-template-go`.**
The v2-idiomatic CLI command; it uses the same canonical template the docs and template CI expect. `-r` runs the template's `init.sh` non-interactively.
Alternatives considered:
- v1.x CLI `crossplane function init` — matches "function init" wording but is from the EOL v1 line; would require pinning an old CLI and produces an older layout that drifts from the current SDK.
- Hand-authoring the module — no scaffolding drift control, loses the canonical template's `input` generation and package layout.
Chosen: `xpkg init` + `init.sh`, then hand-fix the pieces `init.sh` misses.

**D2 — Go module path: `github.com/melak-cmd/crossplane-labs/functions/function-scale`.**
Matches the repo's `source` (`github.com/melak-cmd/crossplane-labs`). Changes via `sed`-style replacements after `init.sh` (module + `fn.go` import + `package/crossplane.yaml`).
Alternative: standalone `github.com/kaonix/function-scale` — cleaner if the function is ever extracted; rejected for now since the PoC lives in-repo.

**D3 — Input type: `Input` with `spec.scaleTargets []ScaleTarget{name string, replicas int32}`.**
Keeps the template's `kind: Input`; the spec's `spec.scaleTargets` maps to a `Spec` struct. `+kubebuilder` markers drive `go generate ./...` (controller-tools, already a module dependency) to regenerate `zz_generated.deepcopy.go` and the input CRD schema in `package/input/`. Group renamed to `function-scale.fn.kaonix.com` (aligns with the platform's `com.kaonix` domain; spec currently says the same after a typo fix).

**D4 — RunFunction behavior mapping.**
- Bootstrap `rsp := response.To(req, response.DefaultTTL)` (passthrough + TTL default 60s = `response.DefaultTTL`) and decode `input` via `request.GetInput`.
- **Empty/missing input or empty `scaleTargets`** → return `rsp` unchanged (no conditions added). This implements the spec's empty-input scenario literally ("no status conditions added beyond Ready: Unknown") and keeps the no-op path trivially idempotent.
- **Non-empty input** → build scale target map; iterate `GetDesiredComposedResources`; for each entry with matching `metadata.name`, `cd.Resource.SetInt("spec.replicas", int64(replicas))`; `SetDesiredComposedResources`; set `Ready=True/Synced=True` (reason `Succeeded`, target composite+claim), emit a normal result.
- **Target present in input but no desired resource matches** → same passthrough, plus a `Synced=False` condition with `reason: TargetNotFound` (message names the target) per the spec; function still succeeds (no fatal).
- Fatal only for internal errors (unreadable input, corrupt desired resources) → `response.Fatal`.

**D5 — Local dev runtime vs packaged deployment.**
- `crossplane render` demo uses the template's dev-runtime annotation (`render.crossplane.io/runtime: Development` in `example/functions.yaml`): renders against the locally built `./function` binary — no cluster, no Docker.
- Real deployment uses a normal `Function` object in `functions/functions.yaml` (`spec.package: registry.localhost:5000/function-scale`), following the repo's local-registry pattern (`project-push`).

**D6 — Repo Makefile targets (mirroring `project-*` naming).**
Add `function-build` (go binary for dev runtime), `function-test`, `function-lint` (golangci-lint, matching template CI), `function-xpkg` (Docker multi-stage + `crossplane xpkg build --package-root=package/`), `function-push` (xpkg push to local registry), `function-render` (render the example). Reuse the template's `Dockerfile` + `.github/workflows/ci.yml` as-is (personalized), leaving the platform CI untouched.

## Risks / Trade-offs

- [Go toolchain drift: template pins `go 1.25.10`, machine has 1.22.2] → Mitigation: rely on `GOTOOLCHAIN=auto` (verified working today); template CI pins the same version. Note in README prerequisites.
- [Scaffolding depends on GitHub template availability] → Mitigation: the shoplifted code is committed; future `crossplane xpkg init` runs are only for new functions.
- [`go generate` needs controller-tools run at build time to regenerate the input schema] → Mitigation: commit the regenerated `package/input/*.yaml` and `zz_generated.deepcopy.go`; `go generate ./...` is a documented step, not a git-hook dependency.
- [Mild spec tension: "successful pass → Ready/Synced True" vs "empty input → no conditions"] → Resolved as D4 (empty/no-target input = explicit no-op with no added conditions); flagged to the user in the apply summary, not silently absorbed.
- [Docker required for `function-xpkg`/push] → Same requirement as the existing `project build`; dev loop (`function-build` + render) needs none.

## Migration Plan

- Deploy: `make function-build && make function-push` then `make install-deps` (or `kubectl apply -f functions/functions.yaml`). The `Function` object is additive — existing functions/providers/compositions unaffected.
- Rollback: remove the `function-scale` `Function` entry from `functions/functions.yaml` (or the whole `functions/function-scale/` dir). No other manifests reference it yet.

## Open Questions

- None that change the spec/approach. (Group choice `function-scale.fn.kaonix.com`, image tag strategy `function-push` uses, and whether to wire the function into a real composition later are all safely deferrable.)