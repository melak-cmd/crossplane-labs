## MODIFIED Requirements

### Requirement: Database API

The Database capability SHALL expose a namespaced Composite Resource of kind `PostgreSQL` in group `database.kaonix.inc.fr` (version `v1alpha1`, plural `postgresqls`) with a required `spec.id` and a required `spec.parameters` object. Parameters SHALL include `namespace` (default `default`), `database` (default `app`), `version` (default `"16"`), `size` (default `small`; supported values `small`, `medium`, `large`), optional `storageSize` string (overrides the size preset when set), optional `storageClass` string (StorageClass for the PostgreSQL PVCs; defaults to the cluster default when unset), and `instances` (default `1`). Parameters SHALL include an optional `backup` object with `schedule` (cron expression, including seconds; required when `backup` is set) and optional `snapshotClass` (the name of the `VolumeSnapshotClass` used for snapshots). When `backup` is absent, no backup resources SHALL be created. The XR status SHALL expose `host`, `port`, `dbName`, and `secretName`, and SHALL expose `backup` with the configured schedule, the ScheduledBackup resource name, and the last successful backup time when CloudNativePG reports one.

#### Scenario: Defaults are applied

- **WHEN** a Database XR with only `spec.id` and `spec.parameters.namespace` is applied
- **THEN** the API server accepts it and applies defaults `database: app`, `version: "16"`, `size: small`, `instances: 1`

#### Scenario: Status fields are populated

- **WHEN** a Database XR is reconciled
- **THEN** its status reports `host`, `port`, `dbName`, and `secretName`

#### Scenario: Backup configuration is optional

- **WHEN** a Database XR without `spec.parameters.backup` is applied
- **THEN** the API server accepts it and no backup resources are created for it

#### Scenario: Backup status is reported

- **WHEN** a Database XR with `spec.parameters.backup` configured is reconciled and CloudNativePG has reported a successful backup
- **THEN** its status reports `backup` with the configured schedule, the ScheduledBackup resource name, and the last successful backup time

### Requirement: Database composition

The Database capability SHALL compose a CloudNativePG `Cluster` (`postgresql.cnpg.io/v1`) as a `kubernetes.m.crossplane.io/v1alpha1` Object in the target namespace using the `default` ProviderConfig, with `instances` from parameters, an initdb bootstrap creating `database` owned by the XR id, `max_connections: "200"`, a `pg_hba` rule trusting the 10.244.0.0/16 range, and storage sized from `storageSize` (falling back to a 1Gi preset) using `storageClass` when set. When the XR sets `spec.parameters.backup`, the composition SHALL configure the Cluster's `spec.backup.volumeSnapshot` stanza (using `className` from `backup.snapshotClass` when set) and SHALL also compose a CloudNativePG `ScheduledBackup` (`postgresql.cnpg.io/v1`) targeting the Cluster with the requested schedule and `method: volumeSnapshot`. The composition SHALL write the XR status (`host` `<id>-rw.<namespace>.svc`, `port` 5432, `dbName`, `secretName` `<id>-app`, and `backup` when configured) through the pipeline, using `function-go-templating` followed by `function-auto-ready`.

#### Scenario: Composition creates the CNPG cluster

- **WHEN** a Database XR is reconciled by the `database-cnpg` composition
- **THEN** a CNPG Cluster named `<id>` is created in the target namespace with the requested instance count, storage size, and initdb bootstrap
- **AND** the XR status is populated and Ready is reported when the cluster is healthy

#### Scenario: Composition creates a scheduled backup

- **WHEN** a Database XR with `spec.parameters.backup` is reconciled by the `database-cnpg` composition
- **THEN** the Cluster is configured for volume-snapshot backups and a ScheduledBackup targeting the composed Cluster is created with the requested schedule and `method: volumeSnapshot`

#### Scenario: Cluster renders identically without backup configuration

- **WHEN** a Database XR without `spec.parameters.backup` is reconciled by the `database-cnpg` composition
- **THEN** the composed CNPG Cluster manifest is identical to the pre-backup rendering (no backup fields) and no ScheduledBackup is composed
