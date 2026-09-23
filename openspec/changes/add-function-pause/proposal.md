## Why

Operators need an explicit, declarative way to pause or resume reconciliation of a specific Crossplane resource during maintenance or recovery work. A Crossplane Operation should invoke a small input-driven function so this capability can be added without changing database Compositions.

## What Changes

- Add a standalone Go function named `function-pause` that can run in a Crossplane Operation pipeline.
- Accept one explicit target reference containing API version, kind, name, and namespace, plus a boolean pause state.
- Apply or remove the `crossplane.io/paused` annotation on the referenced XR through the Operation resource-application path.
- Include unit tests, generated input schema, package metadata, an Operation example, and lifecycle support alongside the existing custom function.
- Leave database and backup Compositions unchanged; scheduled-backup suspension remains outside this change.

## Capabilities

### New Capabilities

- `custom-functions/pause`: Operation-driven pausing and resuming of an explicitly referenced Crossplane resource.

### Modified Capabilities

<!-- No existing capability requirements change. -->

## Impact

- Adds `functions/function-pause/` with Go source, tests, generated input CRD, package metadata, and an Operation example.
- Adds the `function-pause` package to `functions/functions.yaml` and extends Makefile function lifecycle support so `FUNCTION_NAME=function-pause` can build, test, lint, package, and render it.
- Adds an Operation manifest demonstrating pause and resume of a named XR.
- Existing database compositions and the existing `function-scale` package remain compatible and unchanged.