## Purpose

Provides an explicit Crossplane Operation function for preparing and restoring a provider-kubernetes Object that owns a CloudNativePG Cluster.

## ADDED Requirements

### Requirement: Recovery input identifies a provider Object and phase

The recovery function SHALL accept an explicit `prepare` or `restore` mode, a non-empty provider Object name and namespace, a recovery plan name, and recovery settings required by the selected source during restore.

#### Scenario: Valid prepare target
- **WHEN** an Operation supplies `prepare` with a provider Object target and plan name
- **THEN** the function writes the current CNPG manifest to the recovery plan

#### Scenario: Valid restore target
- **WHEN** an Operation supplies `restore` with a provider Object target, plan name, and one valid source
- **THEN** the function recreates the provider Object with recovery bootstrap

### Requirement: Recovery sources are mutually exclusive

A recovery request MUST specify exactly one supported source: an existing CNPG Backup reference or VolumeSnapshot-based recovery. The function SHALL reject requests that specify neither source or both sources.

#### Scenario: Backup reference selected
- **WHEN** the input contains a valid CNPG Backup reference and no VolumeSnapshot source
- **THEN** the function renders a recovery Cluster using the Backup source

#### Scenario: VolumeSnapshot source selected
- **WHEN** the input contains valid VolumeSnapshot recovery details and no Backup reference
- **THEN** the function renders a recovery Cluster using the VolumeSnapshot source

#### Scenario: Both sources selected
- **WHEN** the input contains both a CNPG Backup reference and VolumeSnapshot recovery details
- **THEN** the function rejects the request as ambiguous and renders no recovery Cluster

#### Scenario: No source selected
- **WHEN** the input contains neither a Backup reference nor VolumeSnapshot recovery details
- **THEN** the function rejects the request as incomplete and renders no recovery Cluster

### Requirement: Backup recovery renders explicit CNPG bootstrap

For a request using an existing CNPG Backup reference, the rendered Cluster SHALL contain the explicit CNPG recovery configuration that points to that Backup and SHALL preserve the requested new identity.

#### Scenario: Restore from an existing Backup
- **WHEN** the input references Backup `orders-backup` in the selected namespace
- **THEN** the rendered Cluster uses the CNPG Backup as its recovery source and has no identity or metadata mutation applied to the source Cluster

### Requirement: VolumeSnapshot recovery renders explicit storage recovery

For a request using VolumeSnapshots, the rendered Cluster SHALL contain explicit recovery configuration for the supplied snapshot set and SHALL preserve the requested new identity.

#### Scenario: Restore from VolumeSnapshots
- **WHEN** the input supplies the required data and WAL VolumeSnapshot references
- **THEN** the rendered Cluster uses those snapshots for recovery and does not update the source Cluster or snapshots

### Requirement: Existing managed Cluster is deleted before restore

The workflow SHALL preserve the current manifest before deletion, require the old provider Object and CNPG Cluster to be deleted before restore, and then recreate the provider Object with the same identity. The restore manifest MUST remove `bootstrap.initdb` before adding `bootstrap.recovery`.

#### Scenario: Restore after deletion
- **WHEN** the old provider Object and Cluster are gone and a valid recovery plan exists
- **THEN** the desired response contains the recreated provider Object with recovery bootstrap and no `bootstrap.initdb`

#### Scenario: Recovery is repeated
- **WHEN** the same valid recovery input is rendered repeatedly
- **THEN** the output remains deterministic for the new identity and does not accumulate changes to any source resource

### Requirement: Recovery failures are reported safely

The function SHALL return a non-success condition for invalid references, missing required snapshot details, or unsupported source configuration, and SHALL avoid emitting a partial recovery Cluster for that request.

#### Scenario: Invalid Backup reference
- **WHEN** the Backup reference is incomplete or cannot be resolved
- **THEN** the function reports a recovery error and emits no partial Cluster

#### Scenario: Incomplete VolumeSnapshot details
- **WHEN** required VolumeSnapshot references are missing
- **THEN** the function reports a recovery error and emits no partial Cluster
