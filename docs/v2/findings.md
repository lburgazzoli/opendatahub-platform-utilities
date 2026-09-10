# V2 Adversarial Review Findings

T17 review status: complete. An independent `sol-high` adversarial pass was
attempted; the delegated reviewer returned a partial result because its
`sol-high` runtime was unavailable. The returned findings were independently
verified against [`v2.md`](v2.md),
[`action-error-semantics.md`](../action-error-semantics.md),
[`development.md`](development.md), and the repository agent rules.

## Findings

### F-001 — Public manager/client wrappers were incorrectly classified as prohibited

- Severity: high (review correction)
- Status: rejected after owner clarification
- Owner: none; the v2 design retains this capability
- Evidence:
  - [`v2/pkg/manager/manager.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/pkg/manager/manager.go:1)
    exports a controller-runtime manager wrapper and path getters.
  - [`v2/pkg/client/client.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/pkg/client/client.go:1)
    exports the cache-coherent client that the manager wrapper exposes.
  - Current consumers are concentrated in the example, integration harness,
    and manager wiring; that concentration is expected for a lifecycle and
    cache-coherence capability, but it leaves the invariant under-tested.
  - The cache-coherent client is required: typed reads must be converted
    through the unstructured cache representation while writes continue to use
    the controller-runtime client.
  - The manager wrapper is the current lifecycle boundary that supplies that
    client consistently to controllers and actions; it is not merely a method
    forwarding wrapper.
  - The previous wording in `v2.md` incorrectly treated both wrappers as
    prohibited. The master document now permits wrappers that own this real
    invariant and removes only redundant forwarding wrappers.
- Impact: this is a review correction, not an implementation defect. Removing
  the wrappers would lose a required cache-coherence capability.
- Follow-up: keep the coherent client and manager wrapper, but remove or
  replace only the currently unused manifest/chart path fields and raw option
  plumbing if no consumer requires them. Add tests for the cache invariant and
  manager sharing behavior.

### F-002 — GC validates immutable action configuration after invocation validation

- Severity: medium
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: [`v2/pkg/action/gc/gc.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/pkg/action/gc/gc.go:94)
  calls `resolveRunOptions` before `a.Validate()`. Deploy and the observation
  actions validate first, and `development.md` requires every `Run` to check
  the cached stable configuration before validating invocation inputs or doing
  I/O.
- Impact: an invalid GC action can report a missing run input instead of its
  stable configuration error, making failures inconsistent across actions and
  weakening the constructor-time validation guarantee.
- Remediation: call `a.Validate()` first in `gc.Action.Run`, then resolve run
  inputs, and add a regression test covering invalid action configuration with
  missing invocation inputs.

### F-003 — The v2 module did not use the requested current Helm renderer

- Severity: medium
- Status: resolved in T18 finding-2 remediation
- Owner: none
- Evidence: [`v2/go.mod`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/go.mod:5)
  now resolves the current `@main` revisions of the Helm, Go-template, and
  Kustomize renderers. Their shared engine API and the compatible
  controller-runtime/Kubernetes dependency line were updated together, and
  the affected renderer call sites use the current engine options API.
- Impact: v2 consumers can compile and behave against a different Helm
  renderer API and implementation than the intended current manifest-kit
  integration; fixes made in the current renderer are not covered by v2
  validation.
- Verification: the v2 race test suite, `go vet`, formatting verification, the
  integration test package, and the Helm example tests pass. `make tidy` still
  cannot complete from the v2 module alone because the workspace integration
  tests import the local `testkit/kind` module; this is a workspace-resolution
  limitation rather than a renderer or compile failure.

### F-004 — Discovery documentation had described the wrapper manager inconsistently

- Severity: low (documentation correction)
- Status: resolved by the master-document correction above
- Owner: none
- Evidence: the consolidation section of `v2.md` removes the manager wrapper,
  but the discovery design later says “The wrapper manager creates one
  instance” and repeats that it shares the discoverer (`v2.md`, lines
  2442–2447 and 3649–3652).
- Impact: the previous master text made the valid wrapper behavior look like
  an accidental exception.
- Remediation: the master document now consistently allows a wrapper that owns
  cache-coherent client or lifecycle-sharing invariants. Discovery concurrency
  and invalidation guarantees are unchanged.

### F-005 — Plain-error precedence can be lost after a later advisory delay

- Severity: high
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: [`v2/pkg/action/error.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/pkg/action/error.go:89)
  reconstructs aggregation state from the previous reason, type, and delay,
  but does not preserve whether the previous terminal reason came from a plain
  Go error. A later advisory error can then supply a positive delay at lines
  108–112. This conflicts with the plain-error precedence in
  [`action-error-semantics.md`](../action-error-semantics.md).
- Impact: a plain failure followed by a delayed advisory after-action can be
  returned as a successful delayed requeue, suppressing the normal
  controller-runtime error backoff and changing failure status interpretation.
- Remediation: preserve plain-error precedence in the accumulator and add a
  sequential regression test for a plain before/main error followed by a
  delayed advisory after action.

### F-006 — The Kind integration fixture registers GC as a finally action

- Severity: review correction
- Status: rejected after owner clarification
- Owner: none
- Evidence: [`v2/test/integration/integration_support_test.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/test/integration/integration_support_test.go:110)
  registers GC with `WithAfterAction`. This is an explicit composition choice
  in the integration fixture, not a framework-inferred ordering.
- Impact: none in the framework contract. Action ordering and the rules for
  when GC is safe are controller-author intent; the pipeline preserves the
  configured phase and registration order and does not identify, reorder, or
  special-case GC.
- Follow-up: controllers using desired-set GC should choose and test an order
  that matches their own desired-resource semantics. No generic GC-ordering
  remediation belongs in T18.

### F-007 — Dynamic registrations targeting one GVK are not aggregated

- Severity: high
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: `dynamicwatcher.Registration` carries an event handler and
  predicates, but [`dynamicwatcher.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/pkg/controller/reconciler/dynamicwatcher/dynamicwatcher.go:189)
  keys every configured registration only by `{GVK, routeConfigured}` and
  discards later inputs after the first source is installed.
- Impact: two conditional `Watches` registrations, or a `Watches` and `Owns`
  registration, for the same GVK do not produce two independent sources or a
  combined source. Only the first active handler and predicate set is
  installed, silently changing reconciliation routing.
- Current-implementation comparison: the pre-v2 dynamic ownership action
  intentionally deduplicated framework-managed watches by `{GVK, owned}`,
  preserving separate owned and unmanaged routes. Static programmatic
  `Watches` and `Owns` registrations were passed individually to
  controller-runtime, so same-GVK registrations did not overwrite each
  other. The v2 regression is specific to conditional programmatic
  registrations sharing the dynamic watcher without per-GVK input
  aggregation.
- Remediation: group programmatic registrations by GVK, append each one as a
  `watchInput` containing its handler, predicates, and activation state, and
  install one source per GVK. The source must fan out events to the active
  inputs while preserving each input's handler and predicate semantics.
  `watchKey` should identify the installed GVK source, not each input; retain
  a route component only where framework-managed routes remain semantically
  distinct. Add tests proving one source, multiple inputs, independent
  predicates, and idempotent repeated synchronization.

### F-008 — Capped cleanup requeues omit the cleanup diagnostic event

- Severity: medium
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: [`reconciler.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/pkg/controller/reconciler/reconciler.go:180)
  returns immediately when the requested cleanup delay exceeds the remaining
  deadline, before the event emission switch at lines 191–200.
- Impact: cleanup diagnostics are not emitted on the path that caps a retry
  delay, even though that path is closest to forced finalizer removal and is
  the most useful path to observe.
- Remediation: compute the capped delay, emit the classifier-appropriate
  cleanup event, and return the capped result. Preserve deadline-expiry
  warning behavior and finalizer semantics.

### F-009 — Cleanup outcome branches lack regression coverage

- Severity: medium
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: cleanup tests cover installation, successful completion, advisory
  completion, and an already-expired deadline, but do not cover the blocking,
  non-blocking, delayed, capped-delay, finalizer-update failure, and
  deletion-only paths together.
- Impact: the finalizer safety contract can regress without detection,
  particularly around retaining the finalizer while cleanup is incomplete and
  capping retries at the deadline.
- Remediation: add table-driven lifecycle tests for each error classifier,
  capped requeues, deadline expiry during execution, update failures, and the
  invariant that deletion never enters normal actions.

### F-010 — Kind cleanup clears retry state before provider deletion succeeds

- Severity: medium
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: [`testkit/kind/engine.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/testkit/kind/engine.go:145)
  clears `cluster`, `provider`, and `tempDir` before calling the provider's
  delete method at line 156. Partial-start cleanup also discards temporary
  directory removal errors in [`engine_support.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/testkit/kind/engine_support.go:111).
- Impact: a transient provider deletion failure cannot be retried by a second
  `Close`, and temporary resources can leak without being reported.
- Remediation: retain cleanup state until deletion succeeds, track pending
  filesystem cleanup separately, preserve joined errors, and test provider
  and temporary-directory cleanup retries.

### F-011 — Kubernetes dependency versions diverge across the v2 workspaces

- Severity: medium
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: `v2/go.mod` resolves Kubernetes 0.35.x, while
  [`v2/examples/helm-builder/go.mod`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/examples/helm-builder/go.mod:5)
  resolves `k8s.io/apimachinery` 0.36.4 with the same controller-runtime
  line. Testkit uses another 0.35.x patch set.
- Impact: standalone and workspace builds exercise different Kubernetes type
  sets and can produce confusing IDE or compile-time type mismatches for
  otherwise identical `GroupVersionKind` values.
- Remediation: align Kubernetes and controller-runtime versions across v2,
  testkit, and examples; validate both workspace mode and `GOWORK=off` mode.

### F-012 — The architecture test does not enforce several normative boundaries

- Severity: medium
- Status: accepted for T18 remediation
- Owner: T18 implementation agent (`luna-high`)
- Evidence: [`v2/internal/architecture/architecture_test.go`](/Users/luca-rh/work/dev/openshift-ai/odh-platform-utilities/v2/internal/architecture/architecture_test.go:188)
  checks only the kube/resources, platform, pipeline, and legacy-import
  relationships. It does not assert the other explicit v2 boundaries, such as
  `pkg/option` independence, `api` behavior restrictions, controller-root
  shape, renderer placement, or public manager/client removal.
- Impact: later changes can reintroduce prohibited dependencies while the
  architecture suite remains green.
- Remediation: add focused negative fixtures and explicit checks for the
  remaining normative boundaries, prioritizing the manager/client boundary and
  public package placement.

## Reviewed areas with no accepted finding

- Dynamic watches expose typed and complete-GVK registration, and dynamic
  ownership preserves the established unmanaged and CRD routing. The
  same-GVK configured-registration collision is recorded separately as F-007.
- Dynamic ownership's `"false"` annotation rule is intentional compatibility
  behavior, not a finding.
- Status persistence uses server-side apply, and deletion keeps deploy and
  label-selector GC out of the cleanup lifecycle.
- The v2 unit/race suite, `go vet`, testkit integration package, and Helm
  example package compiled successfully during this review. The live Kind
  path remains environment-dependent and was not rerun because no container
  runtime is available in this workspace.
- The historical `docs/module-operator-scenarios.md` document is explicitly
  labeled as a v1 scenario catalog; its framework imports are therefore not a
  v2 dependency violation. It should be migrated or retained as historical
  documentation in a later documentation task, but it is rejected as a T17
  implementation finding.

## Disposition

Findings F-002 and F-005, and F-007 through F-012 remain accepted for the next
task, T18. F-001 is rejected as a review misclassification after confirming
that typed/unstructured cache coherence is required. F-004 is resolved by the
master-document correction. F-006 is rejected after confirming that GC
ordering is controller-author intent rather than framework semantics. The
historical scenario-catalog concern remains rejected as a v2 code finding.
T17 does not change production code; T18 owns the accepted remediation and
regression coverage, followed by the second adversarial/validation pass
required by the plan.
