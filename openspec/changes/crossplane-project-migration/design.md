## Context

The repo currently packages the platform as a single Configuration via `crossplane xpkg build` (`make build-xpkg`), with dependency metadata hand-written in `crossplane.yaml`. The Crossplane CLI (v2.5, standalone `crossplane/cli` repo) now provides the Control Plane Project format — `crossplane-project.yaml` (`dev.crossplane.io/v1alpha1` `Project`) plus an opinionated tree — and builds projects into packages with `crossplane project build`. Motivation: see proposal.md — Why. The three XRDs/compositions already live at `apis/{apps,databases,networks}/{definition.yaml,composition.yaml}`, which matches the project layout convention for `apis/<capability>/{definition.yaml,composition.yaml}` unchanged. Cluster install currently uses the source-manifest flow (`make install-deps`), which stays.

## Goals / Non-Goals

**Goals:**
- Introduce `crossplane-project.yaml` as the single dev-time metadata source, with all four existing dependencies pinned under `spec.dependencies`.
- Switch packaging/build from `crossplane xpkg` to `crossplane project build` (and optionally `crossplane project push`), retiring `build-xpkg`/`push-xpkg`/`install-xpkg`.
- Keep the on-disk `apis/` file names/layout intact so `make install-deps`, `validate-*`, `render-*`, and uptest keep working unchanged.
- Keep `make setup`/`install-deps` (source manifests, no dependency resolution) as the default install path.

**Non-Goals:**
- Migrating the composition pipelines to embedded functions (`functions/` in the project) or regenerating XRDs/compositions with `crossplane xrd generate`/`composition generate` — content is preserved as-is.
- Replacing the uptest e2e flow with `crossplane project run`/`test run` (wired later, once project tests cover this platform's needs).
- Renaming any capability, XRD kind, or composed resource.

## Decisions

### D1: Project metadata file

Add `crossplane-project.yaml` at repo root:
- `apiVersion: dev.crossplane.io/v1alpha1`, `kind: Project`, `metadata.name: kaonix-platform`, `spec.repository` set to the local registry (`registry.localhost:5000/kaonix-platform`) since no embedded functions exist (repository only drives embedded-function package naming and is required by the schema).
- `spec.dependencies`: the same four pins currently in `crossplane.yaml` (`function-go-templating >=v0.12.0`, `function-auto-ready >=v0.7.0`, `provider-kubernetes >=v1.3.0`, `provider-helm >=v1.3.0`) — now declared declaratively so `crossplane project build` generates correct package metadata and dependency runtime semantics.
- `spec.crossplane.version`: constraint matching the installed server (`>=v2.3.3` or similar) so the built Configuration advertises a compatible Crossplane version requirement.

*Alternative considered:* keep generating `crossplane.yaml` by hand and only add project scaffolding for futures. Rejected — that maintains two sources of truth and forgoes the CLI's dependency/package metadata generation.

### D2: Packaging via `crossplane project build`

Replace `make build-xpkg`/`push-xpkg` with project equivalents:
- `make project-build` → `crossplane project build` (builds the Configuration package from the project).
- `make project-push` → `crossplane project push` (push to `registry.localhost:5000`).
- Remove `install-xpkg` (no Configuration is installed via the package manager in the default flow).

*Alternative considered:* dual-run both `crossplane xpkg` and `crossplane project` flows. Rejected as dead surface area — nothing installs the Configuration package anymore (`make setup` uses `install-deps`).

### D3: Install flow stays source-manifest based

`make setup`/`install-deps` (namespace + `apis/` + `crossplane/functions/` + `crossplane/providers/` + providerconfigs) is unchanged; no package dependency auto-resolution. The project is only the authoring/packaging format, not the runtime install mechanism for local dev.

### D4: VALIDATION

The specs' "Project-based source of truth" requirements (CLI build + smoke render) are validated in CI by adding a `crossplane project build` step alongside the existing validate step; the uptest e2e job remains the behavioral gate.

## Risks / Trade-offs

- [CLI behavior differences for projects without embedded functions] → Verify during implementation with the installed CLI (`crossplane --version`, `crossplane project build` on a scratch branch first); if the local CLI predates project support, upgrade to v2.5 before implementation.
- [`crossplane project build` requires a Docker runtime] → Same constraint the render/validate flow already has; CI already runs Docker.
- [Removing the registry/xpkg flow breaks any external consumer of `kaonix-platform` OCI image] → None exist (single-repo lab); documented in README.
- [Repository field semantic for non-embedded-function projects] → Use a local placeholder; revisit when embedded functions are introduced.

## Migration Plan

1. Add `crossplane-project.yaml` with metadata + the four pinned dependencies.
2. Smoke-test `crossplane project build` (scratch check of CLI version/output) and confirm the produced packages contain the three XRDs + compositions.
3. Delete `crossplane.yaml`.
4. Update Makefile: add `project-build`/`project-push`, remove `build-xpkg`/`push-xpkg`/`install-xpkg`; keep `install-deps`, `setup`, `uptest`, `validate-*`, `render-*`.
5. Update README (packaging section) and `.github/workflows/e2e.yaml` (project build step).
6. Verify: `make validate`, then `make setup` + `make uptest` produce the same passes as the current baseline.
7. Rollback: revert the commits; `crossplane.yaml` remains in git history and the old Makefile targets are restorable in one checkout.

## Open Questions

- Exact `crossplane project build` output layout and whether it emits the single Configuration package cleanly with zero embedded functions (verify in step 2 of the plan; no spec or task breakdown changes based on the result).
- Whether the installed CLI is already the standalone v2.5 binary (confirm with `crossplane --version` before implementing).