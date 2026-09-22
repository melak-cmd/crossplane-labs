# Composition Read Contract

This document is the single source of truth for how the platform's
Compositions relate to the cluster resources they manage downstream — which
resources each Composition composes, which provider identity reads and manages
them, and how their status is observed. It exists because observation of
composition-managed resources is silently dependent on RBAC grants and
ProviderConfigs: scoping either today would break resources silently, and the
wildcard roles that currently hide every grant are load-bearing, not tidy.

The inventory table below is machine-generated and guarded. Do not hand-edit
it — re-run `python3 scripts/validate-rbac.py --render-doc-table` after any
Composition or function-template change, and `make validate-rbac` (or
`... --strict`) to keep this document in sync with the code.

## Downstream resource inventory

Every resource listed below exists because a Composition manifests it through
a `kubernetes.m.crossplane.io/v1alpha1 Object` composited resource. The column
`Conditional` marks resources produced only when a parameter is set on the
composite resource (Go-template `{{ if }}` block). The column `ProviderConfig`
is the ProviderConfig name referenced by that Composition's Objects —
`default` is the platform's in-cluster identity.

### Downstream resource inventory

| Composition | XR | Downstream | apiVersion/kind | Resource | Conditional | ProviderConfig |
| --- | --- | --- | --- | --- | --- | --- |
| app-frontend | kaonix.com/v1alpha1/App | Deployment | apps/v1 | deployments | no | default |
| app-frontend | kaonix.com/v1alpha1/App | HorizontalPodAutoscaler | autoscaling/v1 | horizontalpodautoscalers | no | default |
| database-backup-cnpg | kaonix.com/v1alpha1/DatabaseBackup | Backup | postgresql.cnpg.io/v1 | backups | no | default |
| database-cnpg | kaonix.com/v1alpha1/Database | Cluster | postgresql.cnpg.io/v1 | clusters | no | default |
| database-cnpg | kaonix.com/v1alpha1/Database | ScheduledBackup | postgresql.cnpg.io/v1 | scheduledbackups | yes | default |
| network-fullstack | kaonix.com/v1alpha1/Network | Ingress | networking.k8s.io/v1 | ingresses | yes | default |
| network-fullstack | kaonix.com/v1alpha1/Network | NetworkPolicy | networking.k8s.io/v1 | networkpolicies | no | default |
| network-fullstack | kaonix.com/v1alpha1/Network | Service | v1 | services | no | default |
| network-fullstack | kaonix.com/v1alpha1/Network | Service | v1 | services | yes | default |

#### Source of the inventory

Derived at validation time from:

- `apis/{apps,networks,databases}/*composition*.yaml` — inline
  `crossplane-contrib-function-go-templating` templates and, for
  `app-frontend`, the `kaonix-platformfunction-app` project function
  pipeline step.
- `functions/function-app/01-compose.yaml.gotmpl` — the project function
  template rendering the Deployment and HPA Objects.

#### Per-Composition breakdown

- **app-frontend** (`apps.kaonix.com`) — function pipeline
  (`kaonix-platformfunction-app` + auto-ready). Unconditionally composes an
  `apps/v1` Deployment and an `autoscaling/v1` HorizontalPodAutoscaler.
- **network-fullstack** (`networks.kaonix.com`) — go-templating + auto-ready.
  Unconditionally composes a `networking.k8s.io/v1` NetworkPolicy and a `v1`
  Service. Conditionally composes a `v1` ExternalName Service (`.spec.parameters.dns`)
  and a `networking.k8s.io/v1` Ingress (`.spec.parameters.ingress`).
- **database-cnpg** (`databases.kaonix.com`) — go-templating + auto-ready.
  Unconditionally composes a `postgresql.cnpg.io/v1` Cluster. Conditionally
  composes a `postgresql.cnpg.io/v1` ScheduledBackup
  (`.spec.parameters.backup`) driving volume-snapshot backups.
- **database-backup-cnpg** (`databasebackups.kaonix.com`) — go-templating +
  auto-ready. Unconditionally composes a `postgresql.cnpg.io/v1` Backup
  (`<id>-backup-manual`).

## RBAC grant matrix

The provider identity referenced by each Composition's Objects must hold, on
the ProviderConfig's target cluster, at least these verbs for every downstream
resource type in the inventory above:

| apiGroup | Resource | read (get/list) | watch (near-real-time) |
| --- | --- | --- | --- |
| apps | deployments | required | optional |
| autoscaling | horizontalpodautoscalers | required | optional |
| networking.k8s.io | networkpolicies | required | optional |
| networking.k8s.io | ingresses | required | optional |
| (core) | services | required | optional |
| postgresql.cnpg.io | clusters | required | optional |
| postgresql.cnpg.io | scheduledbackups | required | optional |
| postgresql.cnpg.io | backups | required | optional |

When near-real-time observation is enabled (see the alpha watch path below),
`watch` becomes required for the affected resource types.

Today these grants are satisfied by a **wildcard ClusterRole**
(`apiGroups: ["*"]`, `resources: ["*"]`, `verbs: ["*"]`) for the
provider-kubernetes identity at `providers/kubernetes.yaml:51-57`. Note that
the same wildcard role exists for provider-helm at `providers/helm.yaml:51-57`
even though **no Composition currently uses provider-helm** — it is unused
grant surface.

The lab-specific `kaonix.io/force-observe` annotation relies on this role to
re-read a composed resource on demand. Scoping any of the rows above would
break that (and normal status refresh) **silently** — no error surfaces in the
XR status, the composite resource just goes stale. This is exactly what
`make validate-rbac` exists to catch: it derives the inventory from the
Compositions and asserts the provider ClusterRole rules cover every
(apiGroup, resource, verb) pair above.

## ProviderConfig and identity contract

Every Object the platform Composes references the same ProviderConfig:
**`default`** (`providers/providerconfigs/default.yaml`, namespace
`platform`). Its credentials source is **InjectedIdentity**, which means the
provider pod acts as its own Kubernetes ServiceAccount —

- provider-kubernetes runs as `crossplane-system/provider-kubernetes`
  (`providers/kubernetes.yaml:41-45`),
- that SA is bound to the ClusterRole `provider-kubernetes`
  (the wildcard role; binding at `providers/kubernetes.yaml:59-70`).

Because the identity is **in-cluster**, this contract is only as good as that
SA's role — the RBAC matrix above applies to the SA on the control-plane
cluster.

If a ProviderConfig ever switches to kubeconfig-backed credentials
(`source: Secret`), the grant requirement shifts to the identity in that
kubeconfig **on the remote target cluster**, and the RBAC matrix above
applies there instead. The platform has no such ProviderConfig today; if one
is added, give it a distinct name and extend
`PROVIDERCONFIG_TO_PROVIDER` in `scripts/validate-rbac.py` so the guard
validates it too.

## Observation and staleness contract

Composed resources' status does not arrive instantly. provider-kubernetes
object observation works like this:

- **Default poll cadence**: each Object is re-read at most every `--poll`
  interval, default **10 minutes** with ±10% jitter. While an Object is **not
  Ready**, the provider tightens the cadence to **30s** so failures propagate
  quickly but steady state settles slowly.
- **Per-object override**: set the `crossplane.io/poll-interval` annotation on
  a composed Object (usable from a Composition template). For example, while
  a CNPG Backup is in flight you can force faster sampling with
  `crossplane.io/poll-interval: "30s"` instead of relying on the 30s
  not-Ready window.
- **Immediate reconcile (the fix for "is it done yet?")**: the canonical
  token is `crossplane.io/reconcile-requested-at` on the Object, which the
  provider honours when it sets `status.lastHandledReconcileAt`. The lab's
  `kaonix.io/force-observe` annotation only worked by accident — any added
  annotation trips the desired-state-changed predicate and nudges a reconcile,
  with **no ack semantics**. Prefer the canonical token.
- **Readiness gating**: by default a composed Object is Ready when its
  underlying resource was created successfully (SuccessfulCreate). That can
  make an XR Ready before the real work completes. When XR readiness must
  reflect the underlying resource, gate the Object's readiness — e.g. an
  Object with `spec.readiness.policy: DeriveFromCelQuery` and
  `object.status.phase == "completed"` for CNPG backups — which also keeps the
  30s poll active while the resource is running.
- **Near-real-time (alpha)**: `--enable-watches` (feature gate `watches`) on
  the provider plus `spec.watch: true` on the Object switches to watch-based
  observation. It needs **watch** RBAC on the identity. When watch is denied,
  the symptom is misleading — the provider logs "watch error - probably remote
  cluster api is gone" — so exhaustion of the RBAC matrix above is the first
  thing to check before blaming the cluster.

Rule of thumb: for the platform's volume-snapshot backup work, the 30s
not-Ready cadence plus the canonical reconcile token is enough; only reach for
`crossplane.io/poll-interval` or watch when you have measured that it is not.