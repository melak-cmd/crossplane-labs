## Why

The existing `function-recovery` name is too generic for a function dedicated to PostgreSQL/CNPG recovery. Aligning the source module, Crossplane package identity, and public input API under `function-pg-recovery` makes its purpose clear and keeps the installed and authored identities consistent.

## What Changes

- **BREAKING**: Rename the function's source/module identity and Crossplane package, runtime, and image references from `function-recovery` to `function-pg-recovery`.
- **BREAKING**: Rename the public function input API group from `function-recovery.fn.database.nuagik.sncf.fr` to `function-pg-recovery.fn.database.nuagik.sncf.fr` and update in-repository consumers and examples.
- Preserve recovery workflow behavior; this change is an identity migration, not a change to recovery semantics.

## Capabilities

### New Capabilities
- `function-pg-recovery`: Defines the PostgreSQL recovery function's public identity and its supported recovery behavior.

### Modified Capabilities

## Impact

- Affects the function source tree and Go module/import paths, package metadata, build/deployment configuration, installation manifests, and RBAC/runtime names.
- Affects input API schema and all in-repository compositions, operations, examples, and documentation that refer to the old API group or package identity.
- Existing external consumers using the old package or input API identity must migrate to the new identity.
