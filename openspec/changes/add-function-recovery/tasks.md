## 1. Confirm Contracts and Package Shape

- [x] 1.1 Confirm the supported CNPG API version and exact Backup and VolumeSnapshot recovery fields against the target cluster schema; record the selected shapes in the function input contract and verify the examples use only supported fields.
- [x] 1.2 Create the standalone `functions/function-recovery` module using the repository's custom-function conventions and verify it builds with the existing Go toolchain.
- [x] 1.3 Define the typed recovery input and generated schema for target identity, mutually exclusive Backup source, and VolumeSnapshot source; verify invalid combinations are rejected by input validation tests.

## 2. Implement Recovery Behavior

- [x] 2.1 Implement deterministic validation requiring a non-empty CNPG Cluster target and exactly one complete recovery source during restore; verify missing, ambiguous, and incomplete inputs return non-success responses without partial desired resources.
- [x] 2.2 Create Backup-based recovery directly as a CNPG Cluster using an explicit Backup reference; verify tests assert the target name/namespace and source fields.
- [x] 2.3 Create VolumeSnapshot-based recovery directly as a CNPG Cluster using the required data and WAL snapshot references; verify tests assert both snapshot references and target identity.
- [x] 2.4 Keep Backup and VolumeSnapshot sources read-only and emit only the requested CNPG Cluster target; verify tests show source objects are not mutated or emitted as recovery targets.
- [x] 2.5 Add deterministic repeat-render and invalid-reference tests; verify repeated valid input produces stable output and every failure omits a partial recovery Cluster.

## 3. Narrow Pause Behavior

- [x] 3.1 Remove the `function-pause` cluster `initdb` mutation path while preserving explicit target selection and pause/resume input behavior; verify the pause tests show only `crossplane.io/paused` changes.
- [x] 3.2 Add regression coverage for pause, resume, repeated operations, invalid targets, and unrelated resources; verify no Cluster bootstrap, operational spec, or unrelated metadata fields are modified.

## 4. Packaging, Operations, and RBAC

- [ ] 4.1 Add recovery package metadata, generated artifacts, and the function package bundle entry following the existing function-scale and function-pause conventions; verify package build and render commands succeed.
- [ ] 4.2 Add separate Operation examples for Backup and VolumeSnapshot recovery using direct CNPG Cluster resources; verify each example renders the expected source branch and target identity.
- [x] 4.3 Define and review least-privilege RBAC for observing CNPG Clusters, referencing recovery plans, and applying the direct recovery Cluster; verify `scripts/validate-rbac.py` and manifest validation pass.
- [ ] 4.4 Add any required Makefile or lifecycle targets for build, test, lint, package, and render; verify the documented `FUNCTION_NAME=function-recovery` workflow completes.
- [x] 4.5 Add a delete Operation for the direct CNPG Cluster and its composition-managed provider Object, with a dedicated runtime service account and least-privilege namespaced `get`/`delete` access; verify idempotent deletion and manifest validation.
- [x] 4.6 Add a post-restore cleanup Operation that removes only `spec.bootstrap.recovery`, including its input schema mode and namespaced `get`/`patch` permission; verify unrelated Cluster fields are preserved and repeated cleanup is safe.
- [x] 4.7 Merge recovery preparation and Cluster deletion into one Operation pipeline step that pauses the Database and persists the recovery plan before deletion; verify ordered behavior with a fake Kubernetes client.

## 5. Documentation and Integration Verification

- [x] 5.1 Document recovery input fields, mutually exclusive source rules, target identity requirements, rollback expectations, and structural-only validation of read-only source references; verify the examples and docs agree with the specs.
- [x] 5.2 Document that `function-pause` only controls `crossplane.io/paused` and no longer changes CNPG `initdb` or bootstrap configuration; verify pause and recovery docs do not imply in-place restore.
- [ ] 5.3 Run the focused Go tests, package/render checks, schema validation, and RBAC validation for both functions; verify all targeted checks pass.
- [ ] 5.4 Run the repository's final OpenSpec and integration validation, including Operation rendering where available; verify the managed Cluster manifests and existing Compositions remain unchanged.
