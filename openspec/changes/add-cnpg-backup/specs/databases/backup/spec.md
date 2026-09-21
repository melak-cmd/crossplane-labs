## Purpose

Defines scheduled and on-demand volume-snapshot backups for CloudNativePG databases exposed through the platform's Database API, covering backup scheduling, backup status reporting, and restore guidance.

## ADDED Requirements

### Requirement: Scheduled volume-snapshot backups

The backup capability SHALL create a CloudNativePG `ScheduledBackup` (`postgresql.cnpg.io/v1`) for every Database XR that sets `spec.parameters.backup.schedule`. The ScheduledBackup SHALL target the composed Cluster `<id>` in the target namespace, SHALL use `method: volumeSnapshot`, and SHALL honor `backup.schedule` (cron expression, including seconds). The capability SHALL configure the composed Cluster's `spec.backup.volumeSnapshot` stanza, setting `className` to `backup.snapshotClass` when that field is provided. When no `backup` configuration is set, the capability SHALL NOT create any backup resources and SHALL NOT add any backup configuration to the Cluster. Snapshot retention SHALL NOT be managed by the capability, because CloudNativePG exposes no retention policy for the volume-snapshot method.

#### Scenario: Scheduled backup is created

- **WHEN** a Database XR with `spec.parameters.backup.schedule` is reconciled
- **THEN** a ScheduledBackup targeting the Cluster is created in the target namespace
- **AND** it uses the requested schedule and `method: volumeSnapshot`
- **AND** the Cluster is configured with `spec.backup.volumeSnapshot`

#### Scenario: Snapshot class is applied

- **WHEN** `spec.parameters.backup.snapshotClass` is set
- **THEN** the Cluster's `spec.backup.volumeSnapshot.className` uses that value

#### Scenario: No backup resources without backup configuration

- **WHEN** a Database XR without `spec.parameters.backup` is reconciled
- **THEN** no ScheduledBackup is created and the Cluster manifest contains no backup configuration

### Requirement: On-demand backups

The backup capability SHALL expose a namespaced Composite Resource of kind `DatabaseBackup` in group `kaonix.com` (version `v1alpha1`, plural `databasebackups`) with a required `spec.id` (the id of an existing Database XR). When reconciled, it SHALL create a CloudNativePG `Backup` (`postgresql.cnpg.io/v1`) for the referenced Cluster using `method: volumeSnapshot`. The DatabaseBackup status SHALL report the backup name, phase, and completion time.

#### Scenario: On-demand backup is created

- **WHEN** a DatabaseBackup XR referencing an existing Database id is applied
- **THEN** a CloudNativePG Backup is created for the referenced Cluster using `method: volumeSnapshot`

#### Scenario: Backup status is observed

- **WHEN** the underlying CloudNativePG Backup completes
- **THEN** the DatabaseBackup status reports the backup name, its phase, and its completion time

### Requirement: Restore guidance

The backup capability SHALL provide restore guidance: recovery of a Database from a volume-snapshot backup SHALL be documented as a cluster recovery via the CloudNativePG bootstrap recovery (`bootstrap.recovery` / `restoredFrom` backup) path in the project's examples and documentation. Automated in-place restore SHALL NOT be performed by the capability.

#### Scenario: Restore procedure is documented

- **WHEN** an operator needs to recover a database from a backup
- **THEN** they can follow the documented recovery procedure to bootstrap a CNPG Cluster from a volume-snapshot backup of the referenced Database
