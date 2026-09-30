# function-pg-recovery

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
stores the recovery plan as an annotation on the `PostgreSQLRestore` before
returning success. Delete uses the
runtime service account to delete the native CNPG Cluster, then waits until it
is absent. Restore creates the recovered Cluster through the function runtime's
Kubernetes client and waits for its CNPG `Ready=True` condition. A retry
succeeds when the existing Cluster already has the requested recovery bootstrap;
it rejects a conflicting Cluster without overwriting it. Cleanup uses the same client to remove only
`spec.bootstrap.recovery` from the restored Cluster. Both entrypoints delegate
to the same CLI implementation.

A Crossplane Operation Function that prepares and restores a CNPG `Cluster`
directly from a CNPG `Backup`.

## Migration from `function-recovery`

The rename changes both the Crossplane package identity and the input API group;
it is not an in-place compatible upgrade. Update Operation and other function
consumers to use `function-pg-recovery` and
`function-pg-recovery.fn.database.nuagik.sncf.fr/v1beta1` together with the
package before deploying the renamed function. Build and publish the package
using `make function-xpkg` and `make function-push`, then apply
`install/function-pg-recovery-rbac.yaml` and `install/functions.yaml`.

If rollback is required, restore the previous consumer manifests and the prior
`function-recovery` package and RBAC manifests from the previous release. The
new package does not serve the old input API group.

Recovery can use two ordered Operation steps: `prepare` pauses the Database XR
and stores a copy of the current CNPG Cluster in the
`recovery.database.nuagik.sncf.fr/plan` annotation of the `PostgreSQLRestore`; when it returns
successfully, the next `delete` step removes the old CNPG Cluster. The function
also supports a single `prepare-delete` mode for
the same sequence. `restore` reads the plan annotation from the live
`PostgreSQLRestore`, removes
`spec.bootstrap.initdb`, adds `spec.bootstrap.recovery`, and creates the CNPG
Cluster with the requested identity. After the restored Cluster is ready, run
`cleanup` to remove `spec.bootstrap.recovery` while preserving the rest of the
Cluster spec; do this before resuming the Database XR.

The combined operation pauses the Database XR before deleting its native
Cluster. Before resuming the XR, account for the Database Composition managing
that Cluster again.

## Input

```yaml
apiVersion: function-pg-recovery.fn.database.nuagik.sncf.fr/v1beta1
kind: Input
spec:
  mode: restore
  target:
    namespace: platform
  backup:
    name: orders-backup
    namespace: platform
```

For `prepare`, provide `mode: prepare` and the CNPG Cluster target; it pauses
the Database and stores the plan on the `PostgreSQLRestore` before returning.
For `delete`, provide `mode: delete` and the Cluster target. This deletes the
live CNPG Cluster and is destructive.
For `restore`, provide
`mode: restore`, the same Cluster target,
and a Backup reference with its name and namespace. The Backup
must be in the same namespace as the target. For `cleanup`, provide `mode: cleanup` and the Cluster target; it
removes only `spec.bootstrap.recovery` and succeeds if that field is already
absent. The function validates recovery source fields but does not check that
the referenced source resources exist.

The supported recovery workflow is the WatchOperation in
`operations/database-restore-watch.yaml`: create a `PostgreSQLRestore` (see
`examples/databases/04-database-restore.yaml`) and it runs the ordered steps.
`mode: prepare-delete` performs prepare and delete in one step.

If you author an Operation by hand instead, each function step provides the PostgreSQL XR
as the required resource `postgresql` and a `PostgreSQLRestore` as the required
resource `postgresqlrestore`. The function reads the XR's
`spec.crossplane.resourceRefs` to resolve the CNPG `Cluster` name;
`target.namespace` remains an explicit input. The plan and recovery phase are
stored on and read from the `PostgreSQLRestore` through the function's
Kubernetes client. No ConfigMap is used and the `planName` input is rejected.

### Recovery plan and phase

The plan annotation `recovery.database.nuagik.sncf.fr/plan` holds the sanitized
Cluster manifest as JSON (it is a copy of the Cluster spec and contains no
credentials). The plan must fit in the Kubernetes annotation budget of 256 KiB
for all annotations together; `prepare` fails before changing anything if it
does not. The annotations are kept after the restore as a record.

The annotation `recovery.database.nuagik.sncf.fr/phase` records progress
(`prepared`, `deleted`, `restored`, `cleaned`, `resumed`). Each step runs only
when the phase is its predecessor (`prepare` with no phase, `delete` after
`prepared`, `restore` after `deleted`, `cleanup` after `restored`, `resume`
after `cleaned`) and records the next phase only after it succeeds. A step
invoked in any other phase succeeds as a no-op. This matters because an update
of a watched `PostgreSQLRestore` can make the WatchOperation start another
Operation for it. Updates made while a restore runs do not (the WatchOperation
uses `concurrencyPolicy: Forbid`, and a normal restore produces exactly one
Operation), but an update after the restore finished does, and a step cannot
tell which Operation it belongs to. That extra Operation finds every step
already done and does nothing.

`operations/database-restore-watch.yaml` watches namespaced
`PostgreSQLRestore` requests without requiring a recovery trigger label.
Restore requests may use `metadata.generateName: postgresqlrestore-` when a
unique Kubernetes-generated name is preferred; explicit `metadata.name` remains
supported.
The watched request `spec.name` identifies the PostgreSQL XR, the request
namespace selects its namespace, and `spec.backupName` selects the CNPG Backup
in that namespace. The WatchOperation runs the prepare, delete, restore,
cleanup, and resume stages. Restore request labels are not used as an
activation filter or acknowledgement mechanism. A failed request is not
automatically retried; create a new request after addressing the failure.

See `examples/databases/04-database-restore.yaml` for a `PostgreSQLRestore`
request.
