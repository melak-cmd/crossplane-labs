## Context

See `proposal.md` for motivation and `specs/custom-functions/recovery/spec.md` plus `specs/custom-functions/pause/spec.md` for the externally visible contracts. The repository packages Crossplane functions as standalone Go projects and installs them through a shared function package manifest. The existing pause capability is being developed in a separate change, while this change defines the narrower annotation-only contract that recovery work must rely on.

## Goals / Non-Goals

**Goals:**

- Add a standalone Operation function and package lifecycle for explicit CNPG recovery.
- Save and restore the CNPG Cluster directly in two explicit phases, using exactly one recovery source during restore.
- Preserve the existing Cluster manifest in a recovery-plan ConfigMap, then recreate the same CNPG Cluster only after the old Cluster is deleted.
- Preserve pause as a small annotation-only function and remove any initdb mutation path during implementation.
- Make examples, tests, package metadata, RBAC, and documentation sufficient for review and deployment.

**Non-Goals:**

- Changing the existing database or backup Compositions.
- Performing an in-place CNPG restore, renaming an existing Cluster, or adopting its identity.
- Supporting a third recovery source, automatic source discovery, scheduled recovery, or backup retention policy.
- Suspending CNPG workloads or changing `crossplane.io/paused` semantics beyond the pause function.

## Decisions

- **Use a dedicated `function-recovery` package.** Recovery has different input validation, source APIs, and safety guarantees from pause, so it will not be added as a mode of `function-pause`. The package follows the existing typed-input, protobuf request/response, unit-test, schema, and xpkg conventions.
- **Represent source selection as a tagged input contract.** The input will expose one Backup reference and one VolumeSnapshot recovery shape, and validation will require exactly one branch. This makes both ambiguous requests and missing source data deterministic failures instead of implicit precedence.
- **Use a two-phase direct Cluster workflow.** Prepare copies the observed CNPG Cluster into a recovery-plan ConfigMap. After the old Cluster is deleted, restore removes `bootstrap.initdb`, adds `bootstrap.recovery`, and creates a CNPG Cluster with the same identity through the function runtime's Kubernetes client. An identical retry succeeds; a conflicting existing Cluster is never overwritten.
- **Use CNPG-native recovery fields.** Backup recovery will reference the existing CNPG Backup in the supported bootstrap/recovery form. VolumeSnapshot recovery will render the CNPG snapshot recovery configuration with the required data and WAL snapshot references. The implementation must verify the exact installed CNPG API version and schema during implementation and render tests.
- **Keep source resolution and rendering separate.** Prepare stores a clean Cluster manifest; restore validates one complete source before constructing the replacement Cluster. Any failure returns a non-success response without a partial resource.
- **Use Operation examples as the acceptance path.** Add one example per source, each targeting the requested CNPG Cluster identity and showing the required input fields. Render tests should assert the generated Cluster name, namespace, source fields, and absence of mutations to Backup or VolumeSnapshot sources.
- **Review least-privilege RBAC explicitly.** The package and Operation manifests will document permissions to observe and apply CNPG Clusters and reference recovery plans, while the implementation will avoid write permissions to Backup and VolumeSnapshot source objects.
- **Treat pause cleanup as a separate implementation task in the same change.** Tests will assert pause/resume changes only the standard annotation and that no `initdb` mutation is emitted. Recovery will not call or share mutation logic with pause.

## Risks / Trade-offs

- [Risk] CNPG recovery field names or VolumeSnapshot support differ across installed CNPG versions. -> Mitigation: pin the target API version in examples, verify it against the cluster schema during implementation, and add render tests for the selected version.
- [Risk] Restore may be applied before the old Cluster has been deleted, causing a recovery update to an existing Cluster instead of a fresh bootstrap. -> Mitigation: document and follow the delete-and-wait step before running restore.
- [Risk] A recovery Operation may be granted broader read or apply permissions than necessary. -> Mitigation: document the exact source-resource verbs, keep source resources read-only, and review generated RBAC with the repository's validation script.
- [Risk] The Database XR's provider-kubernetes Object may continue reconciling the old manifest after the XR is paused or resumed. -> Mitigation: delete the old Object before restore and ensure the resumed Composition does not reapply the pre-recovery manifest.
- [Risk] A malformed source could produce a partially rendered Cluster. -> Mitigation: validate the tagged source and all required fields before constructing desired resources; test every invalid branch.
- [Risk] Removing pause's old initdb behavior may expose callers that depended on it. -> Mitigation: make the behavior removal explicit in the pause spec, update examples and documentation, and add regression tests proving annotation-only output.

## Migration Plan

1. Implement and unit-test the standalone recovery function and the pause regression behavior.
2. Generate or update typed input schemas and package metadata, then add the recovery package to the shared function bundle.
3. Add Backup and VolumeSnapshot Operation examples and render them against the repository's supported CNPG/provider setup.
4. Apply the least-privilege RBAC changes and run the repository RBAC validation and function/package checks.
5. Roll back by removing the recovery package and Operation examples; existing managed database resources and Compositions remain unchanged. Restore the previous pause package only if the implementation branch explicitly requires rollback.

## Open Questions

- Confirm during implementation which exact CNPG API version in the target cluster exposes the intended VolumeSnapshot recovery fields; the spec requires the behavior, not a particular internal field mapping.
