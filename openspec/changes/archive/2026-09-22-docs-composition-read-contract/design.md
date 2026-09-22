## Context

Current state (see proposal.md - Why for motivation):

- Four Compositions produce downstream resources through `kubernetes.m.crossplane.io/v1alpha1` Objects: `app-frontend` (project function `kaonix-platformfunction-app` templates `functions/function-app/*.gotmpl` → `apps/v1` Deployment, `autoscaling/v1` HPA), `network-fullstack` (`apis/networks/composition.yaml` → NetworkPolicy, Service, Ingress), `database-cnpg` (Cluster + conditional ScheduledBackup), `database-backup-cnpg` (Backup). Object `providerConfigRef` is `default` everywhere.
- The provider-kubernetes identity is `InjectedIdentity` (provider pod SA, same cluster) acting against ProviderConfig `default` (`providers/providerconfigs/default.yaml`). Its grants come from a wildcard ClusterRole in `providers/kubernetes.yaml:51-57`; provider-helm has an identical wildcard role (`providers/helm.yaml:51-57`) but no Composition uses it.
- Facts to codify: provider-kubernetes reads are direct `client.Get` calls; default poll 10m (±10% jitter), tightened to 30s while the composed Object is not Ready; `crossplane.io/reconcile-requested-at` (acked in `status.lastHandledReconcileAt`) is the canonical immediate-reconcile token; `crossplane.io/poll-interval: "30s"` overrides cadence; alpha `--enable-watches` + `spec.watch: true` needs read+watch grants and fails silently (misleading "remote cluster api is gone" log) when RBAC denies `watch`.
- Makefile `validate` umbrella (Makefile:125) currently only renders/validates compositions (needs Docker) — there is no static check; `.PHONY` list is at Makefile:9-12.

## Goals / Non-Goals

**Goals:**
- A single maintainable doc (`docs/compositions.md`) that states the per-Composition downstream inventory, the RBAC grant matrix, the ProviderConfig/identity contract, and the observation/staleness contract.
- A static, dependency-light validator that derives the downstream inventory from the composition files and function templates (single source of truth = code, not the doc), asserts the provider ClusterRoles cover it, and cross-checks the doc so it cannot silently drift from the code.
- Wire the validator as `make validate-rbac`, part of the `validate` umbrella.

**Non-Goals:**
- No changes to Compositions, CRDs, providers, or runtime behavior. Wildcard roles stay.
- No actual scoping of RBAC — this change only makes the contract visible and checkable.
- No cluster-touching validation (e.g., `kubectl auth can-i`): the guard is a static repo check.
- Leaving other documented hygiene debt alone (stale `functions/functions.yaml` breaking `make render-*`/`validate-*`, provider-helm unused) — separate change.

## Decisions

- **Locate downstream inventory source in code.** The validator parses `apis/{apps,networks,databases}/*composition*.yaml` (valid YAML) and `functions/function-app/*.gotmpl` (Go template with YAML-shaped bodies) rather than an extra hand-maintained manifest file. The doc is then a rendering, not a second source of truth. Alternative (hand-maintained manifest consumed by both doc and validator) rejected: two sources to keep in sync when a Composition changes.
  - Composition YAML → for each `resources[].base` with `kind: Object`, read `spec.forProvider.manifest.apiVersion`/`kind`; also capture `providerConfigRef.name`.
  - Gotmpl → regex for `apiVersion: <gv>` followed on the next line (same indent) by `kind: <Kind>` inside the `forProvider.manifest` blocks; skips the outer `kubernetes.m.crossplane.io/v1alpha1 Object` wrappers.
  - Conditional detection: a resource is marked conditional when its base has a patch whose `toFieldPath` targets `spec.forProvider.manifest` (whole-manifest replacement) or `spec.forProvider.manifest.metadata.name` — this is the pattern used by `network-fullstack`. Approximation; documented as such. (Fallback `--manual-conditional NAME` keeps the rare wrong guess in the doc, not the guard.)
- **Kind → resource name mapping is explicit, not generic pluralization.** Small table: `Deployment→deployments`, `HorizontalPodAutoscaler→horizontalpodautoscalers`, `NetworkPolicy→networkpolicies`, `Ingress→ingresses`, `Service→services`, `Secret→secrets`, `Cluster→clusters`, `ScheduledBackup→scheduledbackups`, `Backup→backups`. Unknown kinds fail loudly so the table is extended deliberately, never silently.
- **Grant check is per (apiGroup, resource, verb).** For each downstream resource the guard asserts the provider-kubernetes ClusterRole (any rule, matching `*` or the specific apiGroup/resource) grants at least `get` and `list` (`watch` when the Composition declares near-real-time observation — none do today, so watch is optional). Wildcard rules pass; scoped rules either pass or the guard prints exact missing (group, resource, verb) rows and exits 1. Providing `--render-doc-table` lets a maintainer (re)generate the inventory table for `docs/compositions.md`.
- **Doc-drift check.** The validator also parses the markdown table in `docs/compositions.md` and compares it to the derived inventory; mismatch is a warning normally, fatal with `--strict`. This is what makes the guard meaningful today while the wildcard roles still pass every grant check.
- **Language: python3, no third-party imports for the gotmpl path; PyYAML only for composition YAML.** Cleanest robust parse, standard on dev machines and CI runners. If PyYAML is missing the script prints an install hint and exits non-zero rather than degrading silently. Alternative: pure shell/`sed` parsing rejected as brittle across indentation and gotmpl structure.
- **Wiring.** `scripts/validate-rbac.py`; Makefile `validate-rbac: ## ...` target invoking it (no Docker), added to `.PHONY` (Makefile:9) and to the `validate` umbrella prerequisites (Makefile:125).

## Risks / Trade-offs

- [Gotmpl parsing is regex-based and can break if template structure changes] → parser pinned to the two manifest blocks; doc-drift `--strict` check catches mismatches the regex can't see; failure mode is a clear error, not silence.
- [PyYAML not available → guard unusable] → explicit non-zero exit with install hint; CI images and the lab (`python3`) both satisfy it today.
- [Wildcard roles make grant checks vacuously pass until RBAC is scoped] → accepted; the doc-drift check still gives the guard value now, and the grant matrix is written so a post-scoping run reports exact gaps.
- [Kind→resource editorial table can drift from real CRD plural names] → unknown kinds fail loudly; table reviewed against CNPG CRDs during implementation.
- [Extra file/target adds maintenance surface] → mitigated by keeping the script single-file, static, and idempotent; it is added to existing `validate` output only.

## Migration Plan

Additive change. Create `docs/compositions.md`, `scripts/validate-rbac.py`, the `validate-rbac` Makefile target; run `make validate-rbac` to confirm green (grant checks pass via wildcard, doc matches derived inventory). Nothing existing changes behavior; removing the target restores the prior Makefile state. No rollback risk.

## Open Questions

- Whether the guard should later also verify grants against a live cluster (`kubectl auth can-i` per resource). Deferred: needs cluster state and is not required to satisfy the specs; the static check is sufficient for the contract.