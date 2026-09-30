## 1. Rename Function Source and Package Identity

- [x] 1.1 Rename the function source directory, Go module path, and internal imports to `function-pg-recovery`; verify with `go test ./...` from the renamed module.
- [x] 1.2 Update Crossplane package metadata, runtime identity, image/build configuration, installation manifests, and related RBAC names; verify package build and manifest validation succeed.

## 2. Migrate Public API and Consumers

- [x] 2.1 Change the public input API group to `function-pg-recovery.fn.database.nuagik.sncf.fr` and regenerate/update generated schema and package artifacts; verify generated artifacts match the API source.
- [x] 2.2 Update in-repository operations, compositions, examples, tests, and documentation to the renamed package and API identities; verify repository searches show no unintended old identity references.

## 3. Validate End-to-End Rename

- [ ] 3.1 Run function tests, project/package builds, and repository validation workflows; verify they pass with the renamed identity and recovery scenarios retain their existing outcomes.
- [x] 3.2 Document the breaking migration and deployment/rollback steps; verify external consumers are instructed to install the renamed package and use the new API group.
