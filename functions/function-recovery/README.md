# function-recovery

## Structure

```text
cmd/function/       CLI and Crossplane gRPC server
main.go             Root build entrypoint required by `crossplane project build`
internal/function/  Request decoding, operation dispatch, and response writing
internal/model/     Runtime model aliases and operation/status names
internal/operations/ Prepare, prepare-delete, restore, delete, and cleanup handlers
internal/cnpg/      CNPG Cluster manifest transforms
internal/kubernetes/ Direct, namespaced CNPG Cluster operations
internal/validation/Recovery input validation
input/v1beta1/      Public typed input and generated schema source
package/            Crossplane function package metadata and generated input schema
```

Prepare reads the referenced Cluster, pauses its Database XR, and synchronously
persists the recovery plan ConfigMap before returning success. Delete uses the
runtime service account to delete the native CNPG Cluster, then waits until it
is absent. Restore creates the recovered Cluster through the function runtime's
Kubernetes client. A retry succeeds when the existing Cluster already has the
requested recovery bootstrap; it rejects a conflicting Cluster without
overwriting it. Cleanup uses the same client to remove only
`spec.bootstrap.recovery` from the restored Cluster. Both entrypoints delegate
to the same CLI implementation.

A Crossplane Operation Function that prepares and restores a CNPG `Cluster`
directly, using exactly one recovery source: a CNPG `Backup` or a pair of data
and WAL `VolumeSnapshot` objects.

Recovery can use two ordered Operation steps: `prepare` pauses the Database XR
and persists a copy of the current CNPG Cluster to a ConfigMap; when it returns
successfully, the next `delete` step removes the old CNPG Cluster. The function
also supports a single `prepare-delete` mode for
the same sequence. `restore` reads the ConfigMap, removes
`spec.bootstrap.initdb`, adds `spec.bootstrap.recovery`, and creates the CNPG
Cluster with the requested identity. After the restored Cluster is ready, run
`cleanup` to remove `spec.bootstrap.recovery` while preserving the rest of the
Cluster spec; do this before resuming the Database XR.

The combined operation pauses the Database XR before deleting its native
Cluster. Before resuming the XR, account for the Database Composition managing
that Cluster again.

## Input

```yaml
apiVersion: function-recovery.fn.kaonix.com/v1beta1
kind: Input
spec:
  mode: restore
  target:
    namespace: platform
  planName: orders-recovery-plan
  backup:
    name: orders-backup
    namespace: platform
```

Use either `backup` or `volumeSnapshots`, never both:

```yaml
volumeSnapshots:
  data: orders-data-snapshot
  wal: orders-wal-snapshot
  storageClass: csi-hostpath-sc
```

For `prepare`, provide `mode: prepare`, the CNPG Cluster target, and
`planName`; it pauses the Database and persists the plan before returning.
For `delete`, provide `mode: delete` and the Cluster target. This deletes the
live CNPG Cluster and is destructive.
For `restore`, provide
`mode: restore`, the same Cluster target,
the plan name, and exactly one complete recovery source. Backup name and
namespace are required. VolumeSnapshot data, WAL, and storage class are all
required. For `cleanup`, provide `mode: cleanup` and the Cluster target; it
removes only `spec.bootstrap.recovery` and succeeds if that field is already
absent. The function validates recovery source fields but does not check that
the referenced source resources exist.

For the initial recovery workflow, use the ordered steps in
`examples/databases/03-prepare-recovery.yaml`, or use `mode: prepare-delete`
with the Cluster target and `planName` to perform the sequence in one step.

Each function step must provide the PostgreSQL XR as the required resource
`postgresql`. The function reads its `spec.crossplane.resourceRefs` to resolve
the CNPG `Cluster` name; `target.namespace` remains an explicit input.

See `examples/databases/03-prepare-recovery.yaml`,
`examples/databases/04-restore-from-backup.yaml`, and
`examples/databases/05-cleanup-recovery-bootstrap.yaml` for the ordered Backup recovery Operations.
