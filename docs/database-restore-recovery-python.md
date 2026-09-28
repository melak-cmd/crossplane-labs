# DatabaseRestore Recovery With function-recovery-python

This guide describes what happens after creating a `DatabaseRestore` custom
resource (XR) that triggers the Python recovery function.

## Start A Restore

Create a request like [04-database-restore.yaml](../examples/databases/04-database-restore.yaml).
The `recovery.kaonix.inc.fr/trigger: requested` label is essential: the
`WatchOperation` selects only requests with that label. `spec.backupName` names
the CNPG `Backup`; `spec.target.namespace` identifies the namespace containing
the PostgreSQL XR, Cluster, Backup, and temporary recovery-plan ConfigMap.

The `DatabaseRestore` CRD is cluster scoped. Its name must therefore match the
target PostgreSQL XR name. The function uses that request name to look up the
PostgreSQL XR in `spec.target.namespace`.

## Main Path

```mermaid
sequenceDiagram
    participant User
    participant Request as DatabaseRestore
    participant Watch as WatchOperation
    participant Function as function-recovery-python
    participant API as Kubernetes API

    User->>Request: Create request with trigger=requested
    Watch->>Function: prepare (watchedRequest=true)
    Function->>API: Read PostgreSQL and Cluster
    Function->>API: Pause PostgreSQL and save Cluster plan ConfigMap
    Watch->>Function: delete
    Function->>API: Delete Cluster and wait for absence
    Watch->>Function: restore
    Function->>API: Create restored Cluster from plan and Backup; wait Ready
    Watch->>Function: cleanup
    Function->>API: Remove spec.bootstrap.recovery
    Watch->>Function: resume
    Function->>API: Unpause PostgreSQL
```

1. [restore-definition.yaml](../apis/databases/restore-definition.yaml) validates
   the `DatabaseRestore` resource. It requires an immutable backup name and
   target namespace.
2. [database-restore-watch.yaml](../operations/database-restore-watch.yaml)
   watches requests labelled `recovery.kaonix.inc.fr/trigger: requested` and
   creates one operation with five ordered pipeline steps: `prepare`, `delete`,
   `restore`, `cleanup`, and `resume`.
3. Every step invokes `function-recovery-python` with `watchedRequest: true`.
   The empty `target` is intentional: the function derives request-specific
   values from the watched `DatabaseRestore`.
4. In each invocation, [fn.py](../functions/function-recovery-python/function/fn.py)
   reads `ops.crossplane.io/watched-resource`, verifies that it is a
   `DatabaseRestore`, removes its trigger label, and derives:
   - PostgreSQL XR name from `metadata.name`.
   - Target namespace from `spec.target.namespace`.
   - Recovery plan name as `<request-name>-recovery-plan`.
   - CNPG Backup name from `spec.backupName` for the `restore` step.
5. The function gets the PostgreSQL XR, resolves its sole CNPG Cluster reference,
   validates the operation input, and dispatches the operation. A missing watched
   resource is a successful no-op; this handles the WatchOperation's synthetic
   deletion event after the trigger label is removed.
6. The `prepare` step pauses reconciliation and saves a sanitized copy of the
   existing Cluster in a ConfigMap. `delete` waits until that Cluster is gone.
7. The `restore` step reads the ConfigMap, adds
   `spec.bootstrap.recovery.backup.name`, creates the Cluster, and waits for its
   `Ready=True` condition.
8. The `cleanup` step removes only `spec.bootstrap.recovery`; `resume` sets the
   PostgreSQL XR's `crossplane.io/paused` annotation to `false`.

## File Roles

| File | Role in the workflow |
| --- | --- |
| [04-database-restore.yaml](../examples/databases/04-database-restore.yaml) | Example request that starts the recovery operation. |
| [restore-definition.yaml](../apis/databases/restore-definition.yaml) | Defines and validates the cluster-scoped `DatabaseRestore` API. |
| [database-restore-watch.yaml](../operations/database-restore-watch.yaml) | Watches labelled requests and defines the ordered recovery pipeline. |
| [functions.yaml](../install/functions.yaml) | Installs the `function-recovery-python` package and associates its runtime configuration. |
| [function-recovery-rbac.yaml](../install/function-recovery-rbac.yaml) | Grants the function service account and Crossplane the access required for requests, XRs, Clusters, and ConfigMaps. |
| [crossplane.yaml](../functions/function-recovery-python/package/crossplane.yaml) | Declares the package as an Operation Function. |
| [function-recovery-python.fn.kaonix.com_inputs.yaml](../functions/function-recovery-python/package/input/function-recovery-python.fn.kaonix.com_inputs.yaml) | Defines the Function input API and supported modes. |
| [main.py](../functions/function-recovery-python/function/main.py) | Starts the gRPC Function server and registers `FunctionRunner`. |
| [fn.py](../functions/function-recovery-python/function/fn.py) | Handles the Function request, derives watched-request input, resolves the PostgreSQL XR, and maps outcomes to operation status. |
| [recovery.py](../functions/function-recovery-python/function/recovery.py) | Parses and validates input; creates the saved plan and the restored Cluster manifest. |
| [operations.py](../functions/function-recovery-python/function/operations.py) | Dispatches the five modes and coordinates the recovery operations. |
| [kubernetes.py](../functions/function-recovery-python/function/kubernetes.py) | Performs Kubernetes API calls, including waits, pause/resume, ConfigMap management, and Cluster lifecycle changes. |
| [status.py](../functions/function-recovery-python/function/status.py) | Writes operation name, status, and message to the Function response output. |
| [constants.py](../functions/function-recovery-python/function/constants.py) | Centralizes response reasons, messages, and operation status values used by `fn.py`. |
| [test_fn.py](../functions/function-recovery-python/tests/test_fn.py) | Tests request parsing, watched-request handling, dispatcher behavior, and response status. |
| [test_kubernetes.py](../functions/function-recovery-python/tests/test_kubernetes.py) | Tests Kubernetes client requests and polling behavior. |
| [test_recovery.py](../functions/function-recovery-python/tests/test_recovery.py) | Tests input validation and recovery-plan/Cluster transformations. |

## Failure Points

- The request must retain the trigger label until it is observed. The function
  removes it only after validating the watched request.
- The target namespace must contain a PostgreSQL XR whose name equals the
  `DatabaseRestore` name and whose resource references contain exactly one CNPG
  Cluster.
- `prepare` must complete before `delete`; otherwise `restore` cannot find the
  `<request-name>-recovery-plan` ConfigMap.
- The named CNPG Backup must be in the same target namespace and usable by
  CloudNativePG for bootstrap recovery.
- The function's service account needs the permissions defined in
   [function-recovery-rbac.yaml](../install/function-recovery-rbac.yaml).
