# Crossplane Project Migration

## Why

The three platform capabilities (App, Database, Network) are hand-authored XRDs and pipeline Compositions under `apis/`, packaged into a single Configuration with `crossplane xpkg build` via bespoke Makefile targets. Crossplane CLI v2.5 introduced the Control Plane Project (`dev.crossplane.io/v1alpha1` `Project`) as the standard way to author, build, and test such platforms: it formalizes the on-disk layout, declares dependencies, scaffolds XRDs/compositions, and builds a project into Configuration + Function packages with `crossplane project build`. Migrating to the project format keeps the platform aligned with upstream tooling instead of maintaining an increasingly divergent custom packaging flow.

## What Changes

- Introduce `crossplane-project.yaml` as the development-time metadata file, replacing `crossplane.yaml` (removed). Provider and function dependencies stay pinned in the project's `spec.dependencies`.
- Reorganize the `apis/` tree into the project layout: each capability gets `apis/<capability>/definition.yaml` (XRD) and `apis/<capability>/composition.yaml`, keeping the current XRD/composition content, pipeline functions (go-templating, patch-and-transform, auto-ready) and functionRef names. API behavior and resource names are unchanged.
- Update the Makefile: switch packaging from `crossplane xpkg` to `crossplane project build` / `push`. Retire the registry-based `build-xpkg`, `push-xpkg`, and `install-xpkg` targets. **BREAKING** — the single `kaonix-platform` Configuration package is replaced by project-built packages.
- Keep the source-manifest `install-deps` flow (direct `kubectl apply` of `apis/`, `crossplane/functions/`, `crossplane/providers/`) as the default cluster setup — no package dependency resolution.
- Add project-native `tests/` scaffolding hooks; the existing uptest e2e flow stays and is pointed at the new layout until `crossplane project run`/`test run` fully replaces it.
- Update README and `.github/workflows/e2e.yaml` to the new targets and layout.

## Capabilities

### New Capabilities

- `apps`: App capability — `kaonix.com/v1alpha1` App XRD and `app-frontend` composition (Deployment + HPA), authored as a Crossplane project (`apis/apps/definition.yaml`, `apis/apps/composition.yaml`) and buildable with the Crossplane CLI.
- `databases`: Database capability — `kaonix.com/v1alpha1` Database XRD and `database-cnpg` composition (CloudNativePG Cluster + connection Secret), authored as a Crossplane project.
- `networks`: Network capability — `kaonix.com/v1alpha1` Network XRD and `network-fullstack` composition (NetworkPolicy, Service, Ingress, ExternalName DNS), authored as a Crossplane project.

### Modified Capabilities

None — `openspec/specs/` is currently empty; all three capabilities are introduced by this change.

## Impact

- Config: `crossplane.yaml` removed, `crossplane-project.yaml` added.
- Layout: `apis/apps/`, `apis/databases/`, `apis/networks/` reorganized into the project structure; `examples/` aligned with project paths.
- Makefile: `build-xpkg`/`push-xpkg`/`install-xpkg` replaced by `crossplane project build`/`push` equivalents; `install-deps`, `setup`, `uptest`, `validate`, `render-*` targets updated where they reference the layout or packaging.
- CI: `.github/workflows/e2e.yaml` updated (CLI-based build/validate; uptest flow unchanged).
- Docs: `README.md` Quick Start, packaging, and Project Layout sections.
- Dependencies: same pins, now declared in the project `dependencies` (function-go-templating >=v0.12.0, function-auto-ready >=v0.7.0, provider-kubernetes >=v1.3.0, provider-helm >=v1.3.0).
- Tooling: requires Crossplane CLI >=v2.5 with `project` commands; Docker needed for `crossplane project build`.