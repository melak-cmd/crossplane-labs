## Context

The Database capability currently exposes a `PostgreSQL` XR (`postgresqls.database.kaonix.inc.fr`) reconciled by the `database-cnpg` pipeline composition (`function-go-templating` + `function-auto-ready`). The composition renders a single CloudNativePG `Cluster` as a `kubernetes.m.crossplane.io/v1alpha1` Object using the `default` provider-kubernetes ProviderConfig, and writes the XR status inline from template variables. The go-templating step runs with `missingkey=error`, so optional fields must be accessed via guarded `index` lookups (the existing `storageSize` pattern). See proposal.md - Why for motivation.

## Goals / Non-Goals

**Goals:**
- Keep the Database XR surface backward compatible: every new field optional; zero backup config renders the exact current cluster manifest.
- Scheduled volume-snapshot backups through the same self-service `Database` abstraction, driven by `spec.parameters.backup`.
- On-demand backups via a new namespaced `DatabaseBackup` XR, implemented purely with project-provided functions (no new providers or external functions).
- Backup observability on both XRs, and a documented restore path without in-place automation.

**Non-Goals:**
- Object-store backups: only the `volumeSnapshot` method is supported (user-confirmed).
- Snapshot retention: CNPG exposes no retention policy for volume snapshots (`ScheduledBackup` has no `retentionPolicy` field, and the Cluster's `backup.retentionPolicy` applies only to `barmanObjectStore`), so `retention` is intentionally not part of the API (user-confirmed).
- A `method` selector: with a single supported method the field is redundant, so the composition always renders `method: volumeSnapshot` (user-confirmed).
- Automated in-place restore (recovery stays an operator-driven procedure documented as a guide).
- Unique-name on-demand backups: the `DatabaseBackup` composes a fixed-name CNPG `Backup`; running another backup requires delete-recreate (user-confirmed).
- Storage-class / CSI-snapshot provisioning and `VolumeSnapshotClass` management — operator-level prerequisites, documented only.

## Decisions

### 1. Embed scheduled backup config in the Database XR; add a separate DatabaseBackup XR for on-demand

Scheduled backups are intrinsic to a database's desired state, so they belong in `spec.parameters.backup`. On-demand backups are one-shot actions with their own lifecycle, so a distinct `DatabaseBackup` XR keeps concerns separate and lets a single Database own multiple backup resources over time (via delete-recreate).

- Alternatives considered: a single generic `Backup` XR for both scheduled and manual (rejected — scheduled cadence duplicates the Database's own desired state and forces cross-resource coordination); operator-managed cluster annotations/sidecar (rejected — not self-service, leaves the `Database` abstraction).

### 2. `spec.parameters.backup` shape and guarded rendering

`backup.schedule` (cron expression, required when `backup` is set) and `backup.snapshotClass` (optional `VolumeSnapshotClass` name). In the template, presence is detected with `index .observed.composite.resource.spec.parameters "backup"` guards and nested `if` blocks so that a Database without backup config produces byte-identical output — preserving backward compatibility and the `missingkey=error` option.

### 3. Composition configures the Cluster and composes a ScheduledBackup only when configured

The same go-templating step adds `spec.backup.volumeSnapshot` to the composed Cluster (`className` set only when `snapshotClass` is provided) and conditionally emits a second Object resource — a CNPG `ScheduledBackup` named `<id>-backup` targeting `cluster: {name: <id>}` in the target namespace, with `schedule`, `method: volumeSnapshot`, and `backupOwnerReference: self`. `function-auto-ready` marks the XR ready when the composed resources are healthy.

### 4. On-demand DatabaseBackup XR with fixed-name Backup

New XRD `databasebackups.database.kaonix.inc.fr` (namespaced, `spec.id` required). A companion pipeline composition renders a CNPG `Backup` Object named `<id>-backup-manual` (distinct from the scheduled `ScheduledBackup` named `<id>-backup`, so both can coexist for the same database; with `setResourceNameAnnotation` so the composed object name is deterministic), `cluster: {name: <id>}`, `method: volumeSnapshot`, in the same namespace as the `DatabaseBackup` XR. Re-running an on-demand backup means deleting and re-applying the `DatabaseBackup` XR (delete-recreate; documented).

### 5. Status observability

- Database XR `status.backup`: the template writes `schedule` and `resourceName` (configured values) and reads `lastSuccessfulBackup` from the observed CNPG Cluster Object via `getComposedResource . "cluster"` plus sprig `dig` into `status.atProvider.manifest.status.lastSuccessfulBackupByMethod.volumeSnapshot` (falling back to `lastSuccessfulBackup`). The value is omitted when CloudNativePG has not reported it.
- DatabaseBackup XR status: patches from the composed Backup Object's `status.atProvider.manifest.status` (`phase`, `startedAt`, `stoppedAt` → `completionTime`, `error`) so the XR reports what CNPG reports.

### 6. Restore documented, not automated

A "Restore / Recovery" guide describes recovering a Database from a volume-snapshot backup by bootstrapping a new CNPG Cluster with `bootstrap.recovery` referencing the `Backup` (`restoredFrom`), or by cloning. Placed in the README/examples; intentionally outside the automatic pipeline because recovery targets a *new* cluster identity and is a deliberate, destructive operator action.

## Risks / Trade-offs

- [Volume snapshots require a CSI driver and a `VolumeSnapshotClass`; the k3d local lab uses local-path storage which does not support snapshots] → Mitigation: documented operator-level prerequisite; `make install-cnpg` unchanged and cluster-side e2e for the snapshot path is documented as a manual prerequisite, while render/validation coverage is automated.
- [Fixed `<id>-backup-manual` name means no concurrent on-demand backups] → Mitigation: documented delete-recreate cycle; acceptable for v0.1 (user-confirmed).
- [No retention for volume snapshots] → Mitigation: retention is deliberately absent from the API and documented as a storage-layer concern.
- [Provider-kubernetes may not surface every CNPG status field] → Mitigation: status reads are defensive (`dig` with defaults); missing fields degrade gracefully without breaking reconciliation.

## Migration Plan

- Ship via the existing project flow: edit `apis/` → `crossplane project build` → push → `make install-deps`. No data migration: all fields optional; existing Database XRs render identically.
- Rollback: revert `apis/` changes and redeploy; optional fields mean no API-level incompatibility in either direction.

## Open Questions

- Whether provider-kubernetes surfaces the CNPG Cluster `status.lastSuccessfulBackupByMethod.volumeSnapshot` through `status.atProvider.manifest` — confirm during apply and adjust the `status.backup` rendering if not (degraded behavior already specified).
