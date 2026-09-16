## Purpose

Provides in-house Crossplane composition functions authored in Go via the Crossplane CLI SDK. The first function — `function-scale` — reads app-name-to-replica mappings from its `Input` and patches desired composed Deployment resources accordingly, acting as a platform learning/PoC for the full authoring → build → render → deploy loop.

## ADDED Requirements

### Requirement: Input schema for scale targets

The function SHALL accept an `Input` resource of `apiVersion: function-scale.fn.kaonix.com/v1beta1` (or the SDK's protobuf-equivalent `RunFunctionRequest` `.input` field) containing a `spec.scaleTargets` list. Each entry MUST have a non-empty `name` (string) and a positive integer `replicas` field.

#### Scenario: Valid input with one target
- **WHEN** `RunFunctionRequest.input` contains `spec.scaleTargets: [{name: "my-app", replicas: 3}]`
- **THEN** the function reads the list and uses it to patch matching desired resources

#### Scenario: Empty input list
- **WHEN** `RunFunctionRequest.input` is `null` or `spec.scaleTargets` is an empty list `[]`
- **THEN** the function returns all desired composed resources unchanged (no errors, no status conditions added beyond `Ready: Unknown`)

---

### Requirement: Patch desired Deployment replicas

For every entry in `scaleTargets`, the function SHALL iterate `RunFunctionRequest.desired.resources` and, for each desired resource whose `.metadata.name` equals the entry's `name`, set `.spec.replicas` to the entry's `replicas` value. Desired resources whose names do not match any entry MUST be returned unchanged.

#### Scenario: Target matches one desired resource
- **WHEN** `scaleTargets` contains `{name: "my-app", replicas: 5}` and `desired.resources` contains a resource with `metadata.name: "my-app"` having `spec.replicas: 1`
- **THEN** the function returns that resource with `spec.replicas: 5`; all other desired resources pass through

#### Scenario: No desired resource matches a target name
- **WHEN** `scaleTargets` contains `{name: "nonexistent", replicas: 3}` and no desired resource has `metadata.name: "nonexistent"`
- **THEN** the function returns `RunFunctionResponse.desired.resources` unchanged (no error); a condition with `reason: "TargetNotFound"` SHALL be recorded in `status.conditions`

#### Scenario: Multiple targets
- **WHEN** `scaleTargets` contains `[{name: "app-a", replicas: 2}, {name: "app-b", replicas: 7}]`
- **THEN** each matching desired resource's `spec.replicas` is set independently; non-matching resources pass through unchanged

---

### Requirement: Response status

The function SHALL return a `RunFunctionResponse` with:
- `meta.ttlSecondsAfterSuccess` set to a configurable value (default: 60)
- `status.conditions` containing at least a `Ready` condition (`type: Ready`, `status: True`, `reason: Succeeded`) and a `Synced` condition (`type: Synced`, `status: True`, `reason: Succeeded`) after a successful pass
- All non-matching desired resources included verbatim in `desired.resources`

#### Scenario: Successful run
- **WHEN** the function completes patching without error
- **THEN** `status.conditions` contains `Ready=True` and `Synced=True`; no `message` field is set (informational only)

#### Scenario: Idempotent run on already-patched resources
- **WHEN** the function is called twice in sequence with the same input and desired resources
- **THEN** the second response is identical to the first; `spec.replicas` retains the previously-set value with no drift

---

### Requirement: Inputs with no-op / passthrough behavior

If a desired composed resource's `metadata.name` does not appear in `scaleTargets`, the function MUST return it exactly as received (no fields modified, no metadata additions).

#### Scenario: Passthrough resource
- **WHEN** `desired.resources` contains `{metadata: {name: "other-app"}, spec: {replicas: 1}}` and `scaleTargets` has no entry for `"other-app"`
- **THEN** the returned `desired.resources` entry is byte-for-byte identical to the input (modulo any required proto normalization)
