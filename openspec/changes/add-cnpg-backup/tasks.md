## 1. API definitions

- [x] 1.1 Add optional `spec.parameters.backup` (`schedule` required when set, optional `snapshotClass`) and `status.backup` (`schedule`, `resourceName`, `lastSuccessfulBackup`) to `apis/databases/definition.yaml`; verify the XRD schema validates (e.g. `crossplane project build` or `kubectl apply --dry-run=client -f apis/databases/definition.yaml`)
- [x] 1.2 Create `apis/databases/backup-definition.yaml` with the `databasebackups.database.kaonix.inc.fr` XRD (namespaced `DatabaseBackup`, required `spec.id`, status `name`/`phase`/`startedAt`/`completionTime`/`error`); verify the XRD is valid and builds

## 2. Composition rendering

- [x] 2.1 Update `apis/databases/composition.yaml` so the go-templating step detects `backup` with a guarded `index` lookup and, only when configured: adds `spec.backup.volumeSnapshot` (with `className` from `snapshotClass` when set) to the Cluster, emits a `ScheduledBackup` Object named `<id>-backup` (`schedule`, `method: volumeSnapshot`, `backupOwnerReference: self`, `cluster.name: <id>`), and writes `status.backup` (`schedule`, `resourceName`, and `lastSuccessfulBackup` read from the observed Cluster Object via `getComposedResource`/`dig` when present); verify a Database without backup config renders byte-identical output and one with backup config renders the ScheduledBackup + `volumeSnapshot` fields
- [x] 2.2 Create `apis/databases/backup-composition.yaml` (pipeline, ref is `DatabaseBackup` `database.kaonix.inc.fr/v1alpha1`) that composes a CNPG `Backup` Object named `<id>-backup-manual` (distinct from the scheduled `<id>-backup`) for `cluster.name: <id>` in the XR namespace with `method: volumeSnapshot`, and patches the XR status from the composed Object's `status.atProvider.manifest.status` (phase, timestamps, error); verify it renders for a sample `DatabaseBackup` XR

## 3. Examples and documentation

- [x] 3.1 Add `examples/databases/01-create-database.yaml` (Database XR with `backup.schedule` and `snapshotClass`) and `examples/databases/02-create-databasebackup.yaml` (DatabaseBackup XR); verify both parse via `kubectl apply --dry-run=client`
- [x] 3.2 Document the restore/recovery procedure (bootstrap the CNPG Cluster from the latest volume-snapshot `Backup` via `bootstrap.recovery`/`restoredFrom`, plus the on-demand delete-recreate cycle) in the README and/or example comments; verify the guide is present and references concrete recovery steps

## 4. Validation and CI integration

- [x] 4.1 Run `crossplane project build` and confirm packages build and contain `postgresqls.database.kaonix.inc.fr`, `databasebackups.database.kaonix.inc.fr`, `database-cnpg`, and `database-backup-cnpg`
- [x] 4.2 Extend `make validate-db` (or add a `render-backup` path) to render the backup-enabled Database and a DatabaseBackup XR through the project functions; verify renders succeed and output contains ScheduledBackup/Backup resources and populated status
- [x] 4.3 Manual e2e performed on the lab cluster for both paths (documented as the alternative to uptest): a backup-enabled Database on the snapshot-capable `csi-hostpath-sc` produced a ScheduledBackup; a `DatabaseBackup` XR produced a CNPG `Backup` (method volumeSnapshot), the VolumeSnapshot provisioned via CSI hostpath (`READYTOUSE=true`), the backup reached `completed`, and `status.backup.lastSuccessfulBackup` / the XR `phase`/`completionTime` were populated. CSI snapshot prerequisite (`storageClass` on a snapshot-capable provisioner) documented in the README
- [x] 4.4 Run the full push/install loop (`make project-build`, `make project-push`, `make install-deps`) on the lab cluster and verify a backup-enabled Database produces a ScheduledBackup, a DatabaseBackup XR produces a CNPG `Backup`, and `status.backup` is populated — `registry.localhost` now resolves (127.0.0.1), so the push/install loop runs clean; verified on the cluster: `ScheduledBackup` `<id>-backup` (schedule + volumeSnapshot), CNPG `Backup` `<id>-backup-manual` completed, and `Database.status.backup` populated (`schedule`, `resourceName`, `lastSuccessfulBackup`)
