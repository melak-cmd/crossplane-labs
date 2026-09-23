## Context

See `proposal.md` for the motivation. The repository already contains `function-scale`, a Go Crossplane function that reads typed input and patches resources. This change invokes a similar function from a Crossplane Operation so database and backup compositions remain unchanged.

## Goals / Non-Goals

**Goals:**

- Reuse the existing custom-function authoring and packaging conventions.
- Make pausing reversible through a boolean target field.
- Invoke the function from a Crossplane Operation rather than a database Composition.
- Use the standard `crossplane.io/paused` annotation so Crossplane honors the pause.

**Non-Goals:**

- Changing database or backup Compositions.
- Deleting, scaling, or otherwise changing a target's operational spec.
- Selecting backups, performing restore operations, or implementing scheduled-backup retention.

## Decisions

- **Use an explicit resource reference.** The Operation input identifies API version, kind, name, and namespace so the function is not coupled to database-specific compositions or resource-name conventions.
- **Patch metadata annotations only.** The function sets `crossplane.io/paused: "true"` for `paused: true` and removes that key for `paused: false`.
- **Use the Operation resource-application path.** The function will render or apply the referenced resource's metadata through the Operation pipeline, using the repository's available Kubernetes provider/function mechanism.
- **Keep database Compositions untouched.** Pause and resume are separate explicit Operations; no pause parameter or function step is added to `apis/databases/composition.yaml`.
- **Treat scheduled-backup suspension as separate.** A CNPG `ScheduledBackup` may have provider-specific suspension semantics that are not equivalent to pausing its Crossplane management object. This function will not infer or mutate `spec.suspend`; compositions can add that behavior in a later, explicit capability.
- **Follow the existing package contract.** The new function will use a typed `Input`, generated deepcopy and input schema, unit tests against protobuf requests, and a package manifest/install entry consistent with `function-scale`.

## Risks / Trade-offs

- [Risk] Pausing an XR stops its Composition reconciliation, so that XR cannot resume itself. -> Mitigation: resume is a separate Operation that directly removes the annotation.
- [Risk] Pausing an XR does not suspend CloudNativePG scheduling or database processes. -> Mitigation: document that this controls Crossplane reconciliation only; scheduled-backup suspension remains separate.
- [Risk] The Operation application mechanism may need provider-kubernetes support for arbitrary target metadata. -> Mitigation: validate the selected pipeline function and RBAC during implementation before packaging the Operation example.
- [Risk] Removing an annotation that a user added independently could be surprising. -> Mitigation: the function owns only the target names supplied in its input; compositions should use it deliberately and document the ownership of the pause annotation.

## Migration Plan

1. Build and unit-test the standalone package.
2. Add its package manifest to the local function installation bundle.
3. Add and render an Operation that pauses and resumes a named XR.
4. Roll back by removing the Operation and package manifest; existing database compositions remain valid because they are unchanged.