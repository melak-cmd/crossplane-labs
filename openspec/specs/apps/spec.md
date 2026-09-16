# Apps Specification

## Purpose

Defines the App capability: a namespaced platform API (`kaonix.com/v1alpha1` App) that deploys a containerized frontend application (Deployment + HPA) through a Crossplane composition, authored and built as a Crossplane CLI project.

## Requirements

### Requirement: App API

The App capability SHALL expose a namespaced Composite Resource of kind `App` in group `kaonix.com` (version `v1alpha1`, plural `apps`) with a required `spec.id` and a required `spec.parameters` object containing a required `image` string, an optional `namespace` string (default `default`), and an optional integer `port` (default `80`).

#### Scenario: Valid App XR is accepted

- **WHEN** an App XR with `spec.id` and `spec.parameters.image` (and no `namespace` or `port`) is applied
- **THEN** the API server accepts it and the defaults (`namespace: default`, `port: 80`) are applied to the composite resource

#### Scenario: Missing required fields are rejected

- **WHEN** an App XR is applied without `spec.id` or without `spec.parameters.image`
- **THEN** the API server rejects it with a validation error

### Requirement: App composition

The App capability SHALL compose a `Deployment` (2 replicas, image and container port from parameters, liveness and readiness HTTP probes on that port, CPU/memory limits 250m/256Mi and requests 125m/128Mi) and a `HorizontalPodAutoscaler` (min 2, max 6, target CPU utilization 80%, scaling the Deployment) as `kubernetes.m.crossplane.io/v1alpha1` Object resources in the target namespace using the `default` ProviderConfig. The composition SHALL use a pipeline of `function-go-templating` (inline GoTemplate) followed by `function-auto-ready`, and SHALL write the target namespace and XR id into the composed names.

#### Scenario: Composition creates Deployment and HPA

- **WHEN** an App XR is reconciled by the `app-frontend` composition
- **THEN** a Deployment named `<id>` and an HPA named `<id>` are created in the target namespace, both labeled `app: <id>`, with the image, port, and scale bounds from the parameters
- **AND** the XR is reported Ready only when both composed resources are ready

### Requirement: Project-based source of truth

The App capability SHALL be authored in the Crossplane project as `apis/apps/definition.yaml` (XRD) and `apis/apps/composition.yaml`, and SHALL validate and package successfully via the Crossplane CLI.

#### Scenario: Project build produces the App package

- **WHEN** `crossplane project build` is run on the project
- **THEN** it succeeds without errors and the produced packages contain the `apps.kaonix.com` XRD and the `app-frontend` composition

#### Scenario: Smoke render in the project layout

- **WHEN** the app example XR is rendered with `crossplane composition render` against `apis/apps/composition.yaml` using the project's functions
- **THEN** the output contains the Deployment and the HorizontalPodAutoscaler with the expected values

### Requirement: Backward-compatible App behavior

Migrating the App capability to the project format SHALL NOT change the XR API, the composed resource names, or the rendered manifest content.

#### Scenario: Existing App XRs keep working

- **WHEN** an App XR that was created before the migration is reconciled after the migration
- **THEN** it produces exactly the same Deployment and HPA (same names, labels, image, probes, and scale bounds) as before