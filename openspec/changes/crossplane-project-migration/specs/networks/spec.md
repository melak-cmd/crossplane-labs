## Purpose

Defines the Network capability: a namespaced platform API (`kaonix.com/v1alpha1` Network) that exposes an application through NetworkPolicy, Service, and optionally ExternalName DNS and Ingress, composed via a Crossplane composition and authored/built as a Crossplane CLI project.

## ADDED Requirements

### Requirement: Network API

The Network capability SHALL expose a namespaced Composite Resource of kind `Network` in group `kaonix.com` (version `v1alpha1`, plural `networks`) with a required `spec.id` and a required `spec.parameters` object. Parameters SHALL include a required `appName` and a required `ports` array (each entry requires `port`, with `protocol` defaulting to `TCP` and optional `targetPort`); optional fields `namespace` (default `default`), `ingress` (optional `host`, `path` default `/`, optional `tlsSecret`), `networkPolicy` (optional `ingress`/`egress` CIDR lists), and `dns` (external name). The XR status SHALL expose `serviceName`, `ingressHost`, and `networkPolicyName`.

#### Scenario: Required parameters are enforced

- **WHEN** a Network XR is applied without `spec.parameters.appName` or without `spec.parameters.ports`
- **THEN** the API server rejects it with a validation error

#### Scenario: Status fields are populated

- **WHEN** a Network XR is reconciled
- **THEN** its status reports `serviceName`, and, when configured, `ingressHost` and `networkPolicyName`

### Requirement: Network composition

The Network capability SHALL compose, as `kubernetes.m.crossplane.io/v1alpha1` Objects in the target namespace using the `default` ProviderConfig: a `NetworkPolicy` (selector `app: <appName>`, policy types Ingress and Egress, rules from the `networkPolicy` parameters or empty lists), a `Service` (selector `app: <appName>`, one port per `ports` entry with protocol defaulting to TCP), an `ExternalName` Service named `<id>-dns` when `dns` is set, and an `Ingress` (host, path default `/`, TLS when `tlsSecret` is set, `ingress.kubernetes.io/ssl-redirect: "false"`, backend pointing at the Service on the first configured port) when `ingress` is set. The composition SHALL use a pipeline of `function-go-templating` followed by `function-auto-ready`.

#### Scenario: Full network (ingress, DNS, network policy)

- **WHEN** a Network XR specifies `ingress`, `dns`, and `networkPolicy` parameters
- **THEN** the NetworkPolicy, Service, ExternalName Service, and Ingress are all created in the target namespace with the requested values, and the XR status exposes the service and ingress host

#### Scenario: Minimal network (no ingress or DNS)

- **WHEN** a Network XR specifies only `appName` and `ports` (no `ingress`, `dns`, or `networkPolicy`)
- **THEN** only the NetworkPolicy (with empty ingress/egress rules) and the Service are created, and the XR is Ready

### Requirement: Project-based source of truth

The Network capability SHALL be authored in the Crossplane project as `apis/networks/definition.yaml` (XRD) and `apis/networks/composition.yaml`, and SHALL validate and package successfully via the Crossplane CLI.

#### Scenario: Project build produces the Network package

- **WHEN** `crossplane project build` is run on the project
- **THEN** it succeeds without errors and the produced packages contain the `networks.kaonix.com` XRD and the `network-fullstack` composition

#### Scenario: Smoke render in the project layout

- **WHEN** the network example XR is rendered with `crossplane composition render` against `apis/networks/composition.yaml` using the project's functions
- **THEN** the output contains the NetworkPolicy, Service, and, when configured, the Ingress and ExternalName Service with the expected values

### Requirement: Backward-compatible Network behavior

Migrating the Network capability to the project format SHALL NOT change the XR API, the composed resource names, or the rendered manifest content.

#### Scenario: Existing Network XRs keep working

- **WHEN** a Network XR that was created before the migration is reconciled after the migration
- **THEN** it produces exactly the same NetworkPolicy, Service, Ingress, and DNS resources as before