## Context

The function currently has a single `function-recovery` identity repeated across its Go module/import paths, Crossplane package metadata and runtime configuration, public input API, install manifests, and in-repository operation/composition references. The required behavior contract is in `specs/function-pg-recovery/spec.md`.

## Goals / Non-Goals

**Goals:**
- Migrate all repository-owned identity references as one coherent rename.
- Keep the existing recovery implementation and request semantics behaviorally equivalent.
- Ensure generated API/package artifacts are regenerated or updated from the renamed source identity.

**Non-Goals:**
- Maintaining a second old API group or package alias. The rename is intentionally breaking.
- Changing recovery sequencing, resource selection, validation, or recovery outcomes.

## Decisions

- **Rename the source directory/module and every repository-owned reference together.** This avoids leaving stale imports, build paths, manifests, or examples that point to a no-longer-published identity. The alternative of renaming only the directory/module would leave the installed and public identities inconsistent.
- **Adopt `function-pg-recovery.fn.database.nuagik.sncf.fr` as the canonical API group and regenerate derived schemas/artifacts from the renamed API source.** This keeps consumers aligned with the new package identity. Retaining the old API group as an alias was considered, but rejected because the requested scope is a complete rename and compatibility aliases would prolong the old identity.
- **Treat this as an identity migration, not a recovery implementation rewrite.** Minimize behavioral risk and validate that existing tests and packaging/build checks continue to pass after reference updates.

## Risks / Trade-offs

- [Existing deployments or external consumers still reference the old package/API identity] → Document the breaking migration and update all repository-owned consumers in the same change; deployment owners must install the renamed package and update external references.
- [Generated files or automation retain old names after source changes] → Search the repository for old identity references and run the established generation, build, and validation workflows.
- [A stale old function installation may coexist with the renamed runtime] → Include deployment guidance to replace the old package/runtime identity rather than treating this as an in-place compatible API change.

## Migration Plan

1. Rename the function module/source identity and update package, runtime, build, and install configuration.
2. Rename the public input API group and regenerate its schema/package artifacts.
3. Update all in-repository operations, compositions, examples, tests, and documentation to use the new identities.
4. Run tests and packaging/manifests validation; confirm no repository-owned `function-recovery` identity references remain where the old identity is meant to be replaced.
5. Deploy the renamed Crossplane package and update any external consumers. Rollback requires restoring the old package/API references and reinstalling the previous package; the two identities are not declared aliases.
