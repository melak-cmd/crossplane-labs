## Why

Operators need an explicit recovery workflow for CloudNativePG databases that can restore into a new, independently managed Cluster without risking mutation of the existing managed Cluster. The current pause function proposal also needs a narrower contract: pausing and resuming should only manage the standard Crossplane annotation, while database bootstrap mutation belongs outside that function.

## What Changes

- Add a standalone `function-recovery` for Crossplane Operations that prepares and restores a provider-kubernetes Object for explicit recovery.
- Support exactly one recovery source per request: an existing CNPG Backup reference or VolumeSnapshot-based recovery; reject ambiguous or incomplete source configuration.
- Ensure recovery deletes the old provider Object before recreating it with the same identity and recovery bootstrap.
- **BREAKING**: Keep `function-pause` limited to adding or removing `crossplane.io/paused`; remove its cluster `initdb` mutation behavior during implementation.
- Add Operation examples for both recovery sources, package metadata, unit and render tests, RBAC review/configuration, and user documentation tasks.
- Preserve existing database Compositions and the original managed Cluster lifecycle.

## Capabilities

### New Capabilities

- `custom-functions/recovery`: Explicit CNPG recovery into a new independent Cluster using either a Backup reference or VolumeSnapshot source.
- `custom-functions/pause`: The pause/resume function contract, limited to the `crossplane.io/paused` annotation and without cluster `initdb` mutation behavior.

### Modified Capabilities

<!-- No capability is present in the main spec tree yet; pause is being finalized by the parallel add-function-pause change. -->

## Impact

- Adds `functions/function-recovery/` with Go implementation, typed input/schema, tests, package metadata, and render/package integration.
- Adds Operation examples and documentation for Backup-based and VolumeSnapshot-based recovery, including the independent identity and source exclusivity guarantees.
- Updates the function package bundle and RBAC considerations for reading or rendering CNPG Backup and VolumeSnapshot resources.
- Updates the existing pause function contract and tests without changing database Composition YAML or mutating the existing managed Cluster.
