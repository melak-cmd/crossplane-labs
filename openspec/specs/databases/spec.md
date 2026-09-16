# Databases Specification

## Purpose

Defines the Database capability: a namespaced platform API (`kaonix.com/v1alpha1` Database) that provisions a CloudNativePG PostgreSQL Cluster (with read-write service, port, database name, and credentials Secret) through a Crossplane composition, authored and built as a Crossplane CLI project.

## Requirements

### Requirement: Database API

The Database capability SHALL expose a namespaced Composite Resource of kind `Database` in group `kaonix.com` (version `v1alpha1`, plural `databases`) with a required `spec.id` and a required `spec.parameters` object. Parameters SHALL include `namespace` (default `default`), `database` (default `app`), `version` (default `"16"`), `size` (default `small`; supported values `small`, `medium`, `large`), optional `storageSize` string (overrides the size preset when set), and `instances` (default `1`). The XR status SHALL expose `host`, `port`, `dbName`, and `secretName`.

#### Scenario: Defaults are applied

- **WHEN** a Database XR with only `spec.id` and `spec.parameters.namespace` is applied
- **THEN** the API server accepts it and applies defaults `database: app`, `version: "16"`, `size: small`, `instances: 1`

#### Scenario: Status fields are populated

- **WHEN** a Database XR is reconciled
- **THEN** its status reports `host`, `port`, `dbName`, and `secretName`

### Requirement: Database composition

The Database capability SHALL compose a CloudNativePG `Cluster` (`postgresql.cnpg.io/v1`) as a `kubernetes.m.crossplane.io/v1alpha1` Object in the target namespace using the `default` ProviderConfig, with `instances` from parameters, an initdb bootstrap creating `database` owned by the XR id, `max_connections: "200"`, a `pg_hba` rule trusting the 10.244.0.0/16 range, and storage sized from `storageSize`, falling back to a 1Gi preset. The composition SHALL write the XR status (`host` `<id>-rw.<namespace>.svc`, `port` 5432, `dbName`, `secretName` `<id>-app`) through the pipeline, using `function-go-templating` followed by `function-auto-ready`.

#### Scenario: Composition creates the CNPG cluster

- **WHEN** a Database XR is reconciled by the `database-cnpg` composition
- **THEN** a CNPG Cluster named `<id>` is created in the target namespace with the requested instance count, storage size, and initdb bootstrap
- **AND** the XR status is populated and Ready is reported when the cluster is healthy

### Requirement: Project-based source of truth

The Database capability SHALL be authored in the Crossplane project as `apis/databases/definition.yaml` (XRD) and `apis/databases/composition.yaml`, and SHALL validate and package successfully via the Crossplane CLI.

#### Scenario: Project build produces the Database package

- **WHEN** `crossplane project build` is run on the project
- **THEN** it succeeds without errors and the produced packages contain the `databases.kaonix.com` XRD and the `database-cnpg` composition

#### Scenario: Smoke render in the project layout

- **WHEN** the database example XR is rendered with `crossplane composition render` against `apis/databases/composition.yaml` using the project's functions
- **THEN** the output contains the CNPG Cluster and the populated XR status

### Requirement: Backward-compatible Database behavior

Migrating the Database capability to the project format SHALL NOT change the XR API, the composed resource names, or the rendered manifest content.

#### Scenario: Existing Database XRs keep working

- **WHEN** a Database XR that was created before the migration is reconciled after the migration
- **THEN** it produces exactly the same CNPG Cluster, connection services, and status fields as before