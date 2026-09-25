# Native Composition Resources

Database resources are composed directly as Kubernetes custom resources. They
are not wrapped in provider-kubernetes `Object`s, so Crossplane creates,
observes, updates, and deletes them using its own service account.

## CNPG Resources

| Composition | XR | Native resource | Resource name | Conditional |
| --- | --- | --- | --- | --- |
| `database-cnpg` | `Database` | `postgresql.cnpg.io/v1 Cluster` | `<id>` | no |
| `database-cnpg` | `Database` | `postgresql.cnpg.io/v1 ScheduledBackup` | `<id>-backup` | when `spec.parameters.backup` is set |
| `database-backup-cnpg` | `DatabaseBackup` | `postgresql.cnpg.io/v1 Backup` | `<id>-backup-manual` | no |

CNPG owns the generated application Secret and Pods; they are not composed
resources. The Go-templating function reads the native `Cluster` and `Backup`
status fields to populate XR status. A manual backup is ready when CNPG reports
`status.phase: completed`; a ScheduledBackup is ready once the resource exists.

## Crossplane RBAC

Crossplane's service account needs access to every CRD it composes. The
aggregated `crossplane-compose-cnpg-resources` ClusterRole in
`operations/rbac.yaml` grants `get`, `list`, `watch`, `create`, `update`,
`patch`, and `delete` on `postgresql.cnpg.io/clusters`,
`postgresql.cnpg.io/scheduledbackups`, and `postgresql.cnpg.io/backups`. Its
`rbac.crossplane.io/aggregate-to-crossplane: "true"` label aggregates these
permissions into Crossplane's main role.

The recovery function has a separate, namespaced Role for its direct CNPG
Cluster and recovery-plan ConfigMap operations. It does not delete or manage a
provider-kubernetes `Object`.