# function-pg-recovery Specification

## Purpose

Defines the public identity and recovery contract of the PostgreSQL/CNPG Crossplane function so that its package and input API clearly identify its domain.

## Requirements

### Requirement: PostgreSQL recovery function identity
The published Crossplane function package, runtime identity, and public input API group SHALL use the `function-pg-recovery` name. The input API group SHALL be `function-pg-recovery.fn.database.nuagik.sncf.fr`.

#### Scenario: Consumer declares an input for the renamed function
- **WHEN** a composition or Operation declares a function-pg-recovery input
- **THEN** the input is addressed using API group `function-pg-recovery.fn.database.nuagik.sncf.fr`

#### Scenario: Function package is installed
- **WHEN** the function package is built and installed through Crossplane
- **THEN** the package and its runtime are identified as `function-pg-recovery`

### Requirement: Preserve PostgreSQL recovery behavior across the rename
The renamed function SHALL retain the existing supported recovery modes, input semantics, validation, sequencing, and observable outcomes, except for the public identity change defined above.

#### Scenario: Existing recovery request uses the renamed input API
- **WHEN** a valid recovery request is submitted using the renamed function-pg-recovery input API
- **THEN** it follows the same supported recovery workflow and produces the same recovery outcomes as before the rename

#### Scenario: Invalid recovery request uses the renamed input API
- **WHEN** an invalid recovery request is submitted using the renamed function-pg-recovery input API
- **THEN** it retains the existing validation and error behavior
