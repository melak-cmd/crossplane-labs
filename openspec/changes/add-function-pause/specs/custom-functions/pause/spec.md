## Purpose

Provides an explicit, input-driven mechanism for a Crossplane Operation to pause or resume one referenced Crossplane resource without changing the resource's Composition.

## ADDED Requirements

### Requirement: Operation pause input identifies one target

The function SHALL accept an `Input` resource with apiVersion `function-pause.fn.kaonix.com/v1beta1`, kind `Input`, and a `spec.target` object. The target MUST contain non-empty `apiVersion`, `kind`, and `name` fields, may contain a `namespace`, and MUST be accompanied by a boolean `spec.paused` field.

#### Scenario: Pause a referenced XR

- **WHEN** an Operation invokes the function with target `{apiVersion: "kaonix.com/v1alpha1", kind: "Database", name: "orders", namespace: "platform"}` and `paused: true`
- **THEN** the Operation SHALL apply `metadata.annotations["crossplane.io/paused"]: "true"` to that target resource

#### Scenario: Resume a referenced XR

- **WHEN** an Operation invokes the function with the same target and `paused: false`
- **THEN** the Operation SHALL remove `metadata.annotations["crossplane.io/paused"]` from that target resource if it exists

### Requirement: Operation targeting is explicit

The function SHALL operate only on the target identified by the input API version, kind, name, and optional namespace. It SHALL not alter database Compositions, unrelated resources, or the target's operational spec.

#### Scenario: Database Composition is unchanged

- **WHEN** an Operation pauses a `Database` XR
- **THEN** the database Composition files and rendered composition pipeline SHALL remain unchanged

### Requirement: Report unmatched targets without destructive changes

The function SHALL report a non-success condition when the target cannot be resolved or updated, and SHALL not delete the target or unrelated resources.

#### Scenario: Target does not exist

- **WHEN** the input references a missing resource named `missing-resource`
- **THEN** the Operation SHALL report `TargetNotFound` and make no destructive changes

### Requirement: Pause input is valid and deterministic

The function SHALL reject an input without a target, with an empty target name, or without a boolean paused value. Repeating the same Operation input SHALL produce the same annotation state without drift.

#### Scenario: Repeated operation

- **WHEN** the same pause or resume Operation is run more than once
- **THEN** the target SHALL retain the requested annotation state and no additional resources SHALL be created