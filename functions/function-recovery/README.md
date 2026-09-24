# function-recovery

A Crossplane Operation Function that recreates a provider-kubernetes `Object`
using exactly one existing recovery source: a CNPG `Backup` or a pair of data
and WAL `VolumeSnapshot` objects.

Recovery is a two-phase workflow. `prepare` copies the current CNPG manifest
from the provider Object into a ConfigMap. Delete the old provider Object and
wait for its CNPG Cluster to disappear. `restore` reads the ConfigMap, removes
`spec.bootstrap.initdb`, adds `spec.bootstrap.recovery`, and recreates the
provider Object with the same identity.

## Input

```yaml
apiVersion: function-recovery.fn.kaonix.com/v1beta1
kind: Input
spec:
  mode: restore
  target:
    name: orders-cluster
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

For `prepare`, provide `mode: prepare`, the provider Object target, and
`planName`. For `restore`, provide `mode: restore`, the same Object target,
the plan name, and exactly one complete recovery source. Backup name and
namespace are required. VolumeSnapshot data, WAL, and storage class are all
required.

See `examples/databases/op-recovery-prepare.yaml`,
`examples/databases/op-recovery-restore.yaml`,
`examples/databases/op-recovery-backup.yaml`, and
`examples/databases/op-recovery-volume-snapshot.yaml` for Operation manifests.
