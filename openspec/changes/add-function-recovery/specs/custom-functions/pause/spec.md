## Purpose

Provides an explicit Crossplane Operation mechanism to pause or resume reconciliation of one referenced resource using only the standard `crossplane.io/paused` annotation.

## ADDED Requirements

### Requirement: Pause and resume manage only the Crossplane annotation

The pause function SHALL add `crossplane.io/paused: "true"` when requested to pause and SHALL remove that annotation when requested to resume. It MUST NOT mutate a Cluster's `initdb`, bootstrap, operational, or unrelated metadata fields.

#### Scenario: Pause a referenced resource
- **WHEN** an Operation requests pause for an explicit target
- **THEN** the target has `crossplane.io/paused: "true"` and no Cluster `initdb` or operational fields are changed

#### Scenario: Resume a referenced resource
- **WHEN** an Operation requests resume for an explicit target
- **THEN** the target's `crossplane.io/paused` annotation is removed and no other target fields are changed

### Requirement: Operation targeting remains explicit

The function SHALL operate only on the target identified by API version, kind, name, and optional namespace. It SHALL not alter database Compositions, CNPG recovery resources, unrelated resources, or an existing Cluster's bootstrap configuration.

#### Scenario: Database target is paused
- **WHEN** an Operation targets a Database resource
- **THEN** only that resource's pause annotation state changes; the Database Composition and Cluster bootstrap configuration remain unchanged

#### Scenario: Unrelated resources are present
- **WHEN** the Operation input identifies one target while other resources are available
- **THEN** unrelated resources pass through without changes

### Requirement: Pause input is valid and deterministic

The function SHALL reject an input without a complete target or without a boolean pause state. Repeating the same input SHALL produce the same annotation state without creating or mutating additional resources.

#### Scenario: Invalid target
- **WHEN** the input omits the target, target name, API version, or kind
- **THEN** the function reports an input error and makes no destructive changes

#### Scenario: Repeated pause or resume
- **WHEN** the same pause or resume input is applied more than once
- **THEN** the target retains the requested annotation state and no additional fields or resources are changed
