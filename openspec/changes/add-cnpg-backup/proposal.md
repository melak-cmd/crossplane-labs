## Why

CloudNativePG clusters provisioned through the `Database` API currently have no backup story: a cluster failure, PVC loss, or accidental deletion means permanent data loss. The platform needs a declarative, self-service backup capability so database owners can schedule volume-snapshot backups and recover when needed, without leaving the Crossplane `Database` abstraction.

## What Changes

- Extend the `Database` XRD (`databases.kaonix.com`) parameters with optional backup configuration: `backup.schedule` (cron expression, required when `backup` is set) and `backup.snapshotClass` (optional `VolumeSnapshotClass` name). Default: no backups are created until explicitly enabled.
- Update the `database-cnpg` composition so that when backup parameters are set it configures the composed `Cluster` for volume-snapshot backups (`spec.backup.volumeSnapshot`) and renders a CloudNativePG `ScheduledBackup` (`postgresql.cnpg.io/v1`) targeting the Cluster with `method: volumeSnapshot`.
- Add a `DatabaseBackup` API (group `kaonix.com`, version `v1alpha1`) enabling on-demand/manual volume-snapshot backups of an existing database (one-shot `Backup` creation) and observability of backup status on both XRs.
- Expose backup identity and status on the Database XR status (`status.backup` with the configured schedule, the ScheduledBackup name, and the last successful backup time when CloudNativePG reports it) via the existing go-templating pipeline.
- Document the restore procedure: recovery of a cluster from a volume-snapshot backup via a `bootstrap.recovery` / `restoredFrom` path. Restore is NOT automated in this change (see Impact).
- Add examples and extend uptest coverage / `make validate-db` rendering to cover the backup-enabled rendering path.

## Capabilities

### New Capabilities
- `databases/backup`: scheduled and on-demand volume-snapshot backups of CloudNativePG databases, backup status reporting, and restore guidance.

### Modified Capabilities
- `databases`: the Database API gains optional `spec.parameters.backup` and `status.backup`; the `database-cnpg` composition configures volume snapshots and renders a `ScheduledBackup` when backups are enabled.

## Impact

- **APIs**: `databases.kaonix.com` XRD schema grows (backward compatible — all new fields optional); new `databasebackups.kaonix.com` XRD for the `DatabaseBackup` XR.
- **Composition**: `apis/databases/composition.yaml` renders a ScheduledBackup and adds `spec.backup.volumeSnapshot` to the Cluster when backups are enabled. Existing clusters for which no backup parameters are set render identically (no behavior change).
- **Dependencies**: relies on the CloudNativePG `ScheduledBackup`/`Backup` CRDs (already available via `install-cnpg`) plus a CSI driver that supports volume snapshots and a `VolumeSnapshotClass`; no new Crossplane providers.
- **Credentials**: none. Volume-snapshot backups do not require object-store credentials.
- **Not in scope**: object-store (`barmanObjectStore`) backups, snapshot retention/lifecycle management (a storage-layer concern — CNPG exposes no retention policy for volume snapshots), automated in-place restore (recovery remains a documented, operator-driven procedure), multi-cloud provider adapters.
- **Project/CI**: `crossplane project build` must still pass; `make validate-db` and uptest cover the new render path.
