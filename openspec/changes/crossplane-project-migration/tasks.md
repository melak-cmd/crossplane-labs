## 1. Prerequisites and Baseline

- [x] 1.1 Confirm the installed Crossplane CLI supports project commands (`crossplane --version`; must be >= v2.5 or a v2.3+ build with `project` support); upgrade to v2.5 if needed. Verify `crossplane project --help` lists `init`, `build`, `push`, `run`, `stop`.
- [x] 1.2 Record the pre-migration baseline on current HEAD. Verify `make validate` exits 0 for all three compositions and `make uptest` reports PASS before any changes are made.

## 2. Project Metadata

- [x] 2.1 Add `crossplane-project.yaml` at the repo root with `apiVersion: dev.crossplane.io/v1alpha1`, `kind: Project`, `metadata.name: kaonix-platform`, `spec.repository: registry.localhost:5000/kaonix-platform`, a Crossplane version constraint, and package metadata (maintainer/source/license/description). Verify the file exists and matches design D1.
- [x] 2.2 Declare the four dependencies in `spec.dependencies`: `function-go-templating >=v0.12.0`, `function-auto-ready >=v0.7.0`, `provider-kubernetes >=v1.3.0`, `provider-helm >=v1.3.0` (type `xpkg`, matching the pins in the old `crossplane.yaml`). Verify the built Configuration package declares the same `dependsOn`.
- [x] 2.3 Run `crossplane project build` for the first time on a branch. Verify it succeeds with a Docker runtime and produces a Configuration package containing exactly the three XRDs (`apps`, `databases`, `networks`) and the three compositions (`app-frontend`, `database-cnpg`, `network-fullstack`); inspect the build output for the produced package list.
- [x] 2.4 Delete `crossplane.yaml`. Verify `crossplane.yaml` is gone from the working tree and no artifact references it except git history.

## 3. Makefile Updates

- [x] 3.1 Add `project-build` (`crossplane project build`) and `project-push` (`crossplane project push`) Makefile targets. Verify `make project-build` succeeds and `make help` lists both targets.
- [x] 3.2 Remove the `build-xpkg`, `push-xpkg`, and `install-xpkg` targets and their .PHONY entries. Verify `make help` no longer lists them and `make -n setup` still resolves `create-cluster install-cnpg install-crossplane install-deps` unchanged.
- [x] 3.3 Keep `install-deps` (namespace + `apis/` + `crossplane/functions/` + `crossplane/providers/` + providerconfigs wait) as the default install path. Verify `make install-deps` is idempotent when re-run against a live cluster.

## 4. Docs and CI

- [x] 4.1 Update `README.md`: replace xpkg packaging instructions with `crossplane project build`/`push`, note that `crossplane.yaml` was replaced by `crossplane-project.yaml`, and remove any references to `build-xpkg`/`install-xpkg`. Verify a repo grep shows no stale `build-xpkg`/`install-xpkg`/`crossplane.yaml` references outside git history or the `openspec/` change.
- [x] 4.2 Add a `crossplane project build` step to `.github/workflows/e2e.yaml` alongside the existing validate step, keeping the uptest e2e leg. Verify the workflow renders without YAML errors and the build step targets the new Makefile target.
- [x] 4.3 Confirm the project layout docs in README list `crossplane-project.yaml` and `apis/<capability>/{definition.yaml,composition.yaml}`. Verify the file tree section matches the actual repo tree.

## 5. Verification

- [x] 5.1 Run `make validate`. Verify all three compositions (app, db, network) validate with exit 0 and no new warnings versus the baseline.
- [x] 5.2 Run the full e2e leg: `make teardown`, `make setup`, apply the three example XRs, and `make uptest`. Verify all XRs reach READY and uptest reports 1 passed, 0 failed (specs' backward-compatible behavior).
- [x] 5.3 Smoke-render each capability in the project layout with `crossplane composition render` (app/database/network example XR + `apis/<cap>/composition.yaml` + functions). Verify the rendered output contains the expected composed resources (Deployment+HPA, CNPG Cluster, NetworkPolicy/Service/Ingress/DNS).