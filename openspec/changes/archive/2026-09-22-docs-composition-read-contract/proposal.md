## Why

Observation of composition-managed resources is silently dependent on the provider identity's RBAC grants and ProviderConfig, but today every grant is hidden behind wildcard ClusterRoles (`*/*/*` in `providers/kubernetes.yaml` and `providers/helm.yaml`). There is no single document that declares what each Composition composes downstream, which verbs the provider identity needs for those resources, or how status staleness behaves (default poll 10m, tightened to 30s while not Ready, `crossplane.io/reconcile-requested-at` token, alpha `spec.watch`). Scoping RBAC today would break observation silently, and the lab's only workaround — a custom `kaonix.io/force-observe` annotation with no ack semantics — is tribal knowledge.

## What Changes

- Add **`docs/compositions.md`** documenting, per Composition:
  - the downstream resources it composes (apiVersion/kind, conditional vs unconditional);
  - the RBAC grant matrix (verbs per downstream resource type) the provider identity must hold;
  - the ProviderConfig/identity contract (InjectedIdentity vs kubeconfig-backed remote clusters);
  - the observation/staleness contract (poll cadence, force-reobserve token, readiness gating) and the alpha watch option.
- Add a new capability spec **`platform-compositions`** encoding the read/observation contract as requirements (document the contract; declare downstream resources per Composition; grant matching read/watch access; provide a validator).
- Add a **guard script** `scripts/validate-rbac.py` (Makefile target `validate-rbac`, wired into the `validate` umbrella) that extracts downstream GVKs from the Compositions and the project function templates, and asserts the provider's ClusterRole rules cover them (`get`/`list` minimum; `watch` where declared). Today the wildcard roles pass; the guard makes silent breakage visible when roles are ever scoped.
- No runtime behavior changes; the existing wildcard roles remain in place.

## Capabilities

### New Capabilities
- `platform-compositions`: declares that each Composition's downstream resources must be documented, that provider identities used for observation must hold matching read (and where declared, watch) grants, and that a validation tool must verify grant coverage against the declared downstream resources.

### Modified Capabilities
- None. Runtime behavior is unchanged; this change only documents and validates existing behavior.

## Impact

- `docs/compositions.md` (new).
- `scripts/validate-rbac.py` (new) and `Makefile` (`validate-rbac` target, `.PHONY`, `validate` umbrella).
- `providers/kubernetes.yaml` / `providers/helm.yaml`: not edited now, but become subject to the guard; the inventory in the doc is derived from `apis/*/composition.yaml` and `functions/function-app/*.gotmpl`.
- No changes to Compositions, CRDs, or providers at runtime.