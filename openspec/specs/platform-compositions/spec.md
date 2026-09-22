# Platform Compositions Specification

## Purpose

Defines the contract for how Compositions relate to the cluster resources they manage downstream — what each Composition documents as its downstream resource types, what read/watch grants the provider identity must hold, and how status observation (staleness) behaves. The purpose is to make observation dependencies explicit so that RBAC scoping or ProviderConfig changes cannot break status propagation silently.

## Requirements

### Requirement: Per-Composition downstream resource inventory

The platform SHALL maintain a document that declares, for each Composition, the downstream resource types it composes (apiVersion and kind, including resources composed only under certain conditions) and the provider identity used to manage them.

#### Scenario: Inventory covers every Composition

- **WHEN** a Composition exists in the platform
- **THEN** the inventory document SHALL list each downstream resource type it composes with its apiVersion and kind

#### Scenario: Conditional resources are declared

- **WHEN** a Composition composes a downstream resource only when a condition is set on the composite resource
- **THEN** the inventory document SHALL mark that resource as conditional

### Requirement: Provider identity read grant coverage for composed resources

For each provider identity referenced by a Composition, the platform SHALL grant get and list access to every downstream resource type in that Composition's inventory, and grant watch access when a Composition declares near-real-time observation. A validator SHALL check that the provider identity's RBAC rules cover the declared downstream resource types.

#### Scenario: Validator passes when grants cover the inventory

- **WHEN** the platform's validator runs against provider RBAC rules that include get and list for every declared downstream resource type (and watch where declared)
- **THEN** the validator exits successfully

#### Scenario: Validator reports a missing grant

- **WHEN** the platform's validator runs against provider RBAC rules that omit a documented downstream resource type
- **THEN** the validator SHALL report the missing resource type and exit with a non-zero status

### Requirement: Status observation contract documented

The platform SHALL document, for composed resources, how their status is observed and refreshed: the default poll cadence, the cadence while a resource is not ready, the mechanism to request an immediate reconcile of an observed resource, and the expectation that composed resources gate their readiness on the underlying resource state.

#### Scenario: Document covers staleness and re-observation

- **WHEN** a user needs to understand or trigger refresh of a composed resource's status
- **THEN** the documented observation contract SHALL describe the default poll cadence, the immediate reconcile mechanism, and any way to alter poll cadence per resource

#### Scenario: Document covers readiness gating

- **WHEN** a user reviews how a Composition reports readiness
- **THEN** the documented observation contract SHALL describe how composed resources gate their readiness against the underlying resource's status