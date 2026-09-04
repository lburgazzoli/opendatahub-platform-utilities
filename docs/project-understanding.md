# ODH Platform Utilities — Project Understanding

This document records the current architecture and runtime model of the
repository. The checked-out code is authoritative; in particular, the
repository currently contains four Go modules and the framework uses the
`ActionError` model described below.

## Purpose and architecture

ODH Platform Utilities is the shared dependency for Open Data Hub module
controllers. ODH uses a hub-and-spoke model:

- The ODH Operator is the hub/orchestrator. It coordinates module runlevels,
  reads module status, aggregates health, and manages upgrade progression.
- Independent module controllers are the spokes. Each generally owns a
  cluster-scoped singleton custom resource and reconciles the resources for
  one platform component.
- This repository defines the contract between the hub and spokes and offers
  reusable Kubernetes/controller-runtime infrastructure.

The important design boundary is that the root module provides composable,
low-level utilities, while `framework/` provides an opinionated complete
controller lifecycle.

## Go modules

The repository currently contains these modules:

| Module | Directory | Role |
| --- | --- | --- |
| `github.com/opendatahub-io/odh-platform-utilities` | `/` | Platform types and low-level Kubernetes/controller utilities |
| `github.com/opendatahub-io/odh-platform-utilities/framework` | `framework/` | Generic reconciler and action pipeline |
| `github.com/opendatahub-io/odh-platform-utilities/framework/testing` | `framework/testing/` | Live-cluster integration-test harness |
| `github.com/opendatahub-io/odh-platform-utilities/flakiness` | `flakiness/` | Test flakiness, runtime, quarantine, and Jira tooling |

`examples/` contains additional standalone example modules. The root Makefile
provides aggregate targets such as `test-all`, `lint-all`, and `all-modules`.
Each module has its own Makefile and Go toolchain pinning convention.

## Platform contract

The root contract is defined in `api/common`. A participating module custom
resource implements `PlatformObject`, which combines Kubernetes object
behavior with accessors for:

- `Status` — phase, conditions, and observed generation
- `Conditions` — detailed health observations
- `ComponentReleaseStatus` — release/version handshake data

The common management state is `Managed` or `Removed`. The mandatory platform
condition types are:

- `Ready` — aggregate health and the primary DAG progression gate
- `ProvisioningSucceeded` — whether desired resources were successfully
  provisioned

`PhaseReady` and `PhaseNotReady` are coarse summaries. `ObservedGeneration`
indicates whether status reflects the current resource generation. A module
should set it only after processing the current generation successfully.

Release status contains named component releases. The special release name
`platform` is used for the platform-version handshake during upgrades.

`api/common/validation` validates behavioral conformance, not merely method
signatures. It verifies that status, condition, release, and phase accessors
actually round-trip changes to the underlying CR fields.

The framework has a separate `framework/api` package with structurally similar
platform types used internally by the framework. These are distinct Go types;
root `api/common` values should not be mixed with framework API values without
an explicit conversion or adapter.

## Root-module utilities

The root module is intended for consumers that want selected utilities without
adopting the full framework.

- `pkg/cluster` handles singleton retrieval, cluster/platform detection, CRD
  checks, ownership metadata, and owner-annotation-based enqueueing.
- `pkg/status` performs status updates with conflict retry.
- `pkg/resources` handles decoding, unstructured conversion, GVK resolution,
  hashing, labels/annotations, ownership, SSA, and dependency-aware ordering.
- `pkg/deploy` provides standalone SSA or patch deployment with caching,
  merge strategies, ordering, customizers, and metrics.
- `pkg/render` provides Helm, Kustomize, and Go-template renderers. Renderers
  produce unstructured Kubernetes resources and have action-pipeline adapters.
- `pkg/controller/conditions` provides lower-level condition management.
- `pkg/controller/gc` provides standalone garbage collection with discovery,
  authorization, predicates, and metrics.
- `pkg/webhook` provides singleton admission helpers.
- `pkg/tls` resolves OpenShift TLS profiles for process TLS and proxy flags,
  with safe non-OpenShift fallback behavior and profile-change watching.
- `pkg/metadata` centralizes platform labels and annotations.

## Framework architecture

The framework is built around `ReconciliationRequest` and an ordered list of
`actions.Fn` functions. Actions share state through the request:

- `Resources` contains rendered unstructured resources.
- `Extensions` carries application-specific values between actions.
- `Conditions` is the current condition manager.
- `Instance`, `Client`, `Controller`, and `Release` provide reconciliation
  context.

A normal module pipeline is typically:

```text
module-specific setup
  -> render Helm/Kustomize/templates
  -> deploy resources
  -> register dynamic ownership and watches
  -> check workload/resource health
  -> garbage-collect stale resources
```

Garbage collection should be last because it uses the final desired resource
set. Modules can inject custom actions when the standard pipeline needs
additional filtering or orchestration.

`ReconcilerBuilder` configures the controller-runtime watches, predicates,
actions, finalizers, release information, condition dependencies, dynamic
ownership, raw event sources, and status behavior.

## Reconciler lifecycle

The framework reconciler performs the following high-level steps:

1. Construct and fetch the module instance.
2. Ensure the instance has a usable GVK.
3. On deletion, run finalizer actions and remove the finalizer only after the
   finalizer pipeline succeeds.
4. Otherwise, add the finalizer when configured and run the apply pipeline.
5. Aggregate the action outcome.
6. Update provisioning and dependent conditions.
7. Recompute and sort the aggregate conditions.
8. Set phase and observed generation based on aggregate happiness.
9. Run the optional post-status hook.
10. Apply status using server-side apply.
11. Return the appropriate error or delayed requeue result.

The framework can also use a default requeue interval for successful
reconciliations that need periodic polling of state not represented by a
Kubernetes watch. The default interval does not apply during deletion.

## Current framework error handling

The authoritative reference is `framework/docs/action-error-semantics.md`.
The implementation is split between:

- `framework/controller/actions/errors/errors.go`
- `framework/controller/reconciler/reconciler_pipeline.go`
- `framework/controller/reconciler/reconciler.go`

An action returns an `error`, but the framework interprets semantic
`ActionError` values along two axes: whether the pipeline continues and
whether provisioning remains successful.

| Outcome | Pipeline | Provisioning | Reconcile behavior |
| --- | --- | --- | --- |
| `nil` | Continue | Successful | Normal success |
| Plain Go error | Stop | Failed | Non-nil error; normal controller-runtime backoff |
| `Terminal` | Stop | Failed | Error, unless a positive explicit delay is attached |
| `NonBlocking` | Continue | Failed | Error, unless a positive explicit delay is selected |
| `Advisory` | Continue | Successful | Optional explicit or default requeue |

`NewActionError`, `NewActionErrorf`, and `NewActionErrorW` create terminal
errors by default. They return values, so classification is expressed by
chaining `.Terminal()`, `.NonBlocking()`, or `.Advisory()`.

### Aggregation rules

`runActions` passes every action result to `ActionError.Add`:

- Plain errors and terminal errors stop the pipeline immediately.
- Non-blocking and advisory outcomes allow later actions to run.
- Diagnostics from earlier outcomes are retained in the aggregate error and
  condition message.
- Non-blocking outcomes outrank advisories.
- The earliest positive semantic requeue delay is selected.
- A plain Go error anywhere in a joined error tree overrides semantic delays
  and uses normal controller-runtime error backoff.
- To deliberately schedule a composite joined failure, classify the entire
  joined error with `NewActionErrorW(...).WithRequeueAfter(...)`.

Controller-runtime ignores `RequeueAfter` when the reconcile method also
returns an error. Therefore, a terminal or non-blocking outcome with an
explicit delay is persisted as failed status and returned as:

```go
ctrl.Result{RequeueAfter: delay}, nil
```

Without a usable delay, the reconciler returns a non-nil error instead.

### Apply semantics

During apply:

- terminal, plain, and non-blocking outcomes mark
  `ProvisioningSucceeded=False`;
- advisory-only outcomes keep provisioning successful but record advisory
  reason/message context;
- pre-apply failure creates a terminal outcome and defaults to a 30-second
  pause (`WithPreApplyRequeueAfter` can override it);
- status is written before the final action outcome is handled;
- a status-write failure is reported as `ReconcileError` and combined with the
  action outcome when one exists.

The aggregate `Ready` condition and phase still depend on the configured
condition aggregator. An advisory does not by itself make the module
`Not Ready`; a non-blocking or terminal provisioning outcome does.

### Deletion semantics

Deletion uses the same outcome machinery for finalizer actions, but does not
write provisioning status:

- a plain or terminal failure stops finalizers, retains the finalizer, and
  returns an error unless it has an explicit delay;
- a delayed terminal/non-blocking outcome retains the finalizer and schedules
  another reconciliation;
- non-blocking failures still allow later finalizer actions to run, but the
  finalizer is retained if the aggregate outcome is unsuccessful;
- advisory outcomes do not block finalizer removal unless they request a
  delayed requeue;
- the finalizer is removed only after a successful, non-requeued pipeline.

This prevents cleanup from being considered complete merely because a legacy
stop marker was returned.

### Deprecated compatibility types

`StopError` and `RequeueAfterError` remain supported as deprecated adapters:

- `StopError` behaves as a terminal action error.
- `RequeueAfterError` continues the pipeline and requests delayed scheduling,
  but remains silent and does not alter provisioning status.

New actions should use `ActionError`. Plain Go errors are appropriate for
unexpected failures; semantic action errors are appropriate when an action
intentionally controls continuation, outcome severity, or retry timing.

## Ownership and garbage collection

Deployment and GC actions use labels, annotations, owner references, and
resource fingerprints to establish desired ownership and remove stale
resources.

Dynamic ownership can discover deployed GVKs and register watches without
requiring every GVK to be declared statically. Safety rules are important:

- Namespaces are excluded from dynamic owner references because deleting a
  module must not cascade-delete an entire namespace.
- CRDs are deployed and watched specially and do not receive ordinary owner
  references.
- Excluded GVKs remain deployable but require explicit watch/ownership
  handling.
- Unmanaged-resource annotations and predicates can opt resources out of
  updates or ownership behavior.

GC is RBAC-aware and supports type/object predicates, unremovable GVK
safelists, propagation policy, and metrics.

## Rendering and deployment data flow

Render actions read manifest/chart/template inputs from the request and append
unstructured objects to `Resources`. Deploy actions consume that slice,
normalize GVKs, apply platform metadata, optionally sort resources by apply
order, and apply them through SSA or patch mode.

Deployment supports per-GVK customizers for cases such as preserving user-set
Deployment replicas/resources or merging special ClusterRole/observability
resources. Fingerprint caches can skip unchanged deployments.

## TLS behavior

`pkg/tls` separates process TLS from operand/proxy TLS:

- process startup uses `Load` and `ConfigFromProfile`;
- proxy command-line flags use `FromAPIServer` with the requested version
  format;
- missing APIServer or non-OpenShift environments use the intermediate
  profile;
- transient startup API failures use the intermediate profile and remain
  watchable;
- unexpected errors such as forbidden access fail startup;
- profile changes trigger manager restart because Go cannot safely change
  minimum TLS version/ciphers on an already-listening server.

Consumers must register the OpenShift API scheme and request the corresponding
RBAC permissions.

## Integration test harness

`framework/testing/integration` is a live-cluster PR-gate helper, not an OLM
installer. The expected environment already contains a ready ODH Operator,
DSCInitialization, and the target namespace.

The harness can:

1. apply an optional operator manifest;
2. create a `DataScienceCluster` with selected managed components;
3. wait for the module CR to have `Ready=True` and
   `ProvisioningSucceeded=True`;
4. wait for a named controller Deployment to have ready replicas;
5. set the module CR to `Removed`, wait for scale-down, and delete the DSC.

The operator remains installed. A dedicated cluster is required because the
harness owns the lifecycle of the configured DSC fixture.

## Flakiness module

The separate `flakiness` module processes OpenShift CI test artifacts:

```text
GCS artifacts
  -> JUnit parsing
  -> normalized TestResult records
  -> Prometheus TSDB
  -> PromQL/runtime/flakiness analysis
  -> quarantine decisions
  -> optional Jira bugs/comments
```

It supports retrying GCS reads, partial scrape failures, JUnit failure
classification properties, persistent quarantine history, runtime percentiles,
suite totals, runtime trends, timeout budgets, and Jira token-expiry checks.

## Important invariants

- Preserve the `api/common` JSON tags and kubebuilder markers; they define the
  platform wire contract and CRD schemas.
- Treat `Ready`, `ProvisioningSucceeded`, phase, and observed generation as
  orchestrator-facing API, not local implementation details.
- Use the framework error classifier deliberately: plain errors stop and use
  normal backoff; `NonBlocking` is the explicit opt-in to continue after a
  failed action; `Advisory` means contextual but successful provisioning.
- Keep GC after all resource-producing actions.
- Do not assign module owner references to Namespaces or CRDs.
- Use the project Makefiles for checks; the root Makefile pins the Go toolchain
  from `go.mod` and aggregates checks across modules.
- Prefer existing root/framework helpers over introducing duplicate constants,
  resource conversion, condition, or ownership logic.

## Key references

- Platform contract: `docs/platform-object-contract.md`
- Framework action errors: `framework/docs/action-error-semantics.md`
- Module scenarios: `docs/module-operator-scenarios.md`
- Integration testing: `docs/integration-testing.md`
- TLS: `docs/module-tls.md`
- Migration guide: `docs/migration-from-operator.md`
- Versioning: `docs/VERSIONING.md`
