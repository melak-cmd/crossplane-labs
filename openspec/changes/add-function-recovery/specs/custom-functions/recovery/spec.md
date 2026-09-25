## Purpose

Provides an explicit Crossplane Operation function for preparing and restoring a CloudNativePG Cluster directly.

## ADDED Requirements

### Requirement: Recovery input identifies a CNPG Cluster and phase

The recovery function SHALL accept `prepare`, `prepare-delete`, `delete`, `restore`, or `cleanup` mode and a non-empty CNPG Cluster name and namespace. Prepare, prepare-delete, and restore require a recovery plan name; restore also requires recovery settings for exactly one source.

#### Scenario: Valid prepare target
- **WHEN** an Operation supplies `prepare` with a CNPG Cluster target and plan name
- **THEN** the function writes the current CNPG Cluster manifest to the recovery plan

#### Scenario: Valid restore target
- **WHEN** an Operation supplies `restore` with a CNPG Cluster target, plan name, and one valid source
- **THEN** the function creates a CNPG Cluster with recovery bootstrap

#### Scenario: Delete target Cluster and its manager
- **WHEN** an Operation supplies `delete` with a CNPG Cluster target
- **THEN** the function deletes the namespaced provider-kubernetes Object named `<cluster-name>-cluster` and the `postgresql.cnpg.io/v1` Cluster with the target name
- **AND** it waits until both resources are absent so the Object cannot recreate the Cluster
- **AND** it does not require a recovery plan or recovery source
- **AND** deleting an already absent Cluster succeeds

#### Scenario: Delete cannot accept recovery sources
- **WHEN** an Operation supplies `delete` together with a Backup or VolumeSnapshot source
- **THEN** the function rejects the request without deleting a Cluster

#### Scenario: Prepare and delete in one Operation step
- **WHEN** an Operation supplies `prepare-delete` with a target Cluster and plan name
- **THEN** the function pauses the Database XR, persists a sanitized Cluster manifest in the recovery-plan ConfigMap, then deletes the managing provider-kubernetes Object and Cluster
- **AND** it does not rely on Operation desired resources being applied between pipeline steps

#### Scenario: Cleanup recovery bootstrap after restore
- **WHEN** an Operation supplies `cleanup` with a target CNPG Cluster after restore
- **THEN** the function removes only `/spec/bootstrap/recovery` using a JSON Patch
- **AND** it preserves all other Cluster fields, including `spec.bootstrap.initdb` if present
- **AND** cleanup succeeds without mutation when the recovery field is already absent

### Requirement: Recovery sources are mutually exclusive

A recovery request MUST specify exactly one supported source: an existing CNPG Backup reference or VolumeSnapshot-based recovery. The function SHALL reject requests that specify neither source or both sources.

The function SHALL validate source fields structurally and SHALL NOT check whether the referenced Backup or VolumeSnapshot resources exist.

#### Scenario: Backup reference selected
- **WHEN** the input contains a valid CNPG Backup reference and no VolumeSnapshot source
- **THEN** the function creates a recovery Cluster using the Backup source

#### Scenario: VolumeSnapshot source selected
- **WHEN** the input contains valid VolumeSnapshot recovery details and no Backup reference
- **THEN** the function creates a recovery Cluster using the VolumeSnapshot source

#### Scenario: Both sources selected
- **WHEN** the input contains both a CNPG Backup reference and VolumeSnapshot recovery details
- **THEN** the function rejects the request as ambiguous and renders no recovery Cluster

#### Scenario: No source selected
- **WHEN** the input contains neither a Backup reference nor VolumeSnapshot recovery details
- **THEN** the function rejects the request as incomplete and renders no recovery Cluster

### Requirement: Backup recovery renders explicit CNPG bootstrap

For a request using a CNPG Backup reference, the rendered Cluster SHALL contain the explicit CNPG recovery configuration that points to that Backup and SHALL preserve the requested target identity.

#### Scenario: Restore from an existing Backup
- **WHEN** the input references Backup `orders-backup` in the selected namespace
- **THEN** the rendered Cluster uses the CNPG Backup as its recovery source and has no identity or metadata mutation applied to the source Cluster

### Requirement: VolumeSnapshot recovery renders explicit storage recovery

For a request using VolumeSnapshots, the rendered Cluster SHALL contain explicit recovery configuration for the supplied snapshot set and SHALL preserve the requested target identity.

#### Scenario: Restore from VolumeSnapshots
- **WHEN** the input supplies the required data and WAL VolumeSnapshot references
- **THEN** the rendered Cluster uses those snapshots for recovery and does not update the source Cluster or snapshots

### Requirement: Existing managed Cluster is deleted before restore

The workflow SHALL preserve the current Cluster manifest before deletion, require the old CNPG Cluster to be deleted before restore, and then recreate the CNPG Cluster with the same identity. The restore manifest MUST remove `bootstrap.initdb` before adding `bootstrap.recovery`.

#### Scenario: Restore after deletion
- **WHEN** the old Cluster is gone and a valid recovery plan exists
- **THEN** the function creates the CNPG Cluster with recovery bootstrap and no `bootstrap.initdb`

#### Scenario: Recovery is repeated
- **WHEN** the same valid recovery input is rendered repeatedly
- **THEN** an existing Cluster with the same recovery bootstrap is accepted as a retry without mutation
- **AND** a conflicting existing Cluster is rejected without being overwritten

### Requirement: Recovery failures are reported safely

The function SHALL return a non-success condition for incomplete target or source fields and unsupported source configuration, and SHALL avoid emitting a partial recovery Cluster for that request. It does not verify source-resource existence.

#### Scenario: Invalid Backup reference
- **WHEN** the Backup reference is incomplete
- **THEN** the function reports a recovery error and emits no partial Cluster

#### Scenario: Incomplete VolumeSnapshot details
- **WHEN** required VolumeSnapshot references are missing
- **THEN** the function reports a recovery error and emits no partial Cluster
