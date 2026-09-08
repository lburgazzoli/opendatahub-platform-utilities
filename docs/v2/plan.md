# V2 Implementation Plan

Status: **implementation in progress; T00–T16 and T21–T24 complete**

The canonical design is [`v2.md`](v2.md). Each task in [`tasks/`](tasks/)
must be executable by an agent starting with clean context. Tasks own
non-overlapping implementation areas and include their own verification.

## Agent policy

- Implementation and remediation: `luna-high`.
- Independent adversarial review: `sol-high`.
- Every agent rereads `AGENTS.md`, this plan, and `v2.md` before changing code.
- No task may weaken or reinterpret `v2.md`; unresolved ambiguity goes into
  `findings.md` and blocks dependent work.
- A task is complete only when its required tests and deterministic checks pass.

## Task graph

```text
T00 audit
  -> T01 isolated Kind engine
  -> T02 v2 module foundation
       -> T03 API       ┐
       -> T04 options   │
       -> T05 kube      ├─ parallel foundation work
       -> T06 platform  │
       -> T07 errors    ┘
            -> T08 pipeline
                 -> T09 deploy ┐
                 -> T10 GC     ├─ parallel action work
                 -> T11 support┘
                      -> T12 reconciler
T08 -> T13 render integration runs in parallel with T09–T12
T12 + T13 -> T14 consumer migration
T12 + T13 -> T21 standalone controller examples
T01 + T12 + T14 -> T15 Kind integration suite
all implementation tasks -> T16 legacy removal
T16 -> T22 condition manager
T16 -> T23 OpenShift TLS integration
T16 -> T24 cluster and distribution discovery
T22 + T23 + T24 -> T17 adversarial review (sol-high)
T17 -> T18 remediation (luna-high)
T18 -> T19 final validation
T19 -> T20 closeout
```

Tasks may run in parallel only when their declared package/file ownership does
not overlap. T01 is the first implementation task and must be independently
buildable; the v2 runtime must not depend on it.

## Verification gates

1. Foundation tasks pass focused unit tests and static architecture checks.
2. Action tasks prove programmatic `Run` and pipeline `Execute` parity.
3. Reconciler behavior is covered by unit tests and real-cluster tests where
   Kubernetes API behavior is required.
4. The Kind suite uses only the isolated engine and public v2 APIs.
5. `sol-high` reviews the completed implementation and task boundaries.
6. `luna-high` fixes accepted findings, adds regression tests, and triggers a
   second review/validation pass.
7. Final validation covers all modules, race tests, lint, formatting, tidy,
   generated code, architecture rules, and Kind integration tests.

## Documentation-pack validation

- [x] Master relocation and relative-link correction checked.
- [x] All 24 task files exist and have objective, verification, and dependency
  sections.
- [x] Local adversarial pass corrected the renderer-to-pipeline dependency in
  T13 and the task graph.
- [x] `sol-high` completed the deploy-focused adversarial review for
  architecture, clarity, cleanliness, and performance.
- [x] Accepted deploy findings were remediated by focused commits
  `765920c`, `19d6058`, `ff6ee84`, and `1d5f7d2`; intentional legacy
  Deployment merging and unsupported legacy-owner cleanup were left unchanged.

## Completion record

- [x] Master proposal moved to `docs/v2/v2.md`.
- [x] Relative links from the moved proposal corrected.
- [x] Clean-context implementation task files created.
- [x] Parallel execution and dependency rules documented.
- [x] Kind-engine isolation requirements documented.
- [x] Adversarial review and remediation gates documented.
- [x] T00 consumer audit committed.
- [x] T01 isolated Kind engine committed.
- [x] T02 v2 module foundation committed.
- [x] T03 v2 API contract committed.
- [x] T04 options and configuration committed.
- [x] T05 Kubernetes primitives committed.
- [x] T06 platform behavior committed.
- [x] T07 action error contract committed.
- [x] T08 action pipeline committed.
- [x] T09 deploy action committed.
- [x] Deploy-focused adversarial review and remediation committed.
- [x] Deploy lifecycle simplified to one current-object lookup per resource in
  `1d5f7d2`; focused tests verify the lookup/apply counts.
- [x] Deploy orchestration split into lookup, ownership, customization, and
  apply phases in `b03c277`.
- [x] One-use ownership policy helper inlined with explicit switch cases in
  `80e008c`.
- [x] Deploy phase boundaries documented in `70c8cc6`.
- [x] Deploy allocation benchmarks split into preparation, cache, apply, and
  no-op-client layers in `409b1d8`.
- [x] Typed-versus-unstructured SSA and Run benchmarks added in `36a1a7b`.
- [x] Resource access and deploy now use unstructured values end to end;
  typed apply remains supported only at the shared resource boundary, and the
  redundant apply deep copy was removed in `7a351f0`.
- [x] SSA ownership policy is explicit at callers, and `deployOne` is colocated
  with run orchestration in `210a84b`.
- [x] T10 GC action implemented with policy-aware desired-set cleanup,
  authorization filtering, static/dynamic discovery, and focused tests in
  `8d9639e`.
- [x] T11 supporting actions implemented with safe selectors, isolated
  pipeline adapters, OpenShift ImageStream parsing, release discovery, and
  focused tests in `5bdd036`.
- [x] T12 controller integration implemented with handlers, predicates,
  reconciler lifecycle, status/finalizer handling, dynamic ownership, and
  GVK resource helpers in `798bada`.
- [x] T13 renderer integration committed in `8e0705d`.
- [x] T21 framework/pipeline standalone Helm controller example and isolated
  Kind integration committed in `74ec1c6`; the plain-controller comparison is
  deferred.
- [x] The `sol-high` T21 adversarial review findings were remediated in
  `5fe3c33`, including namespace-scoped caching, safe Kind cleanup, aggregate
  integration validation, README alignment, and the shared generic
  `reconciler.Instance` request accessor.
- [x] T21 integration tests remain in the builder module, the chart lives under
  `config/chart` and is copied to `/opt/charts` in the image, and the example
  uses the compatible `renderer-helm@main` manifest-kit dependency set.
- [x] T14 moved the live-cluster harness from `framework/testing` to the
  standalone `testkit/integration` module, migrated it to v2 APIs, and added
  `testkit/go.work` for local v2, Kind, and integration-harness development.
- [x] T14 validation passed: workspace module resolution, race-tested unit
  tests, `go vet`, and root-configured golangci-lint for `testkit/integration`.
- [x] T15 added the isolated v2 Kind integration suite in
  `v2/test/integration`, with a v2 workspace linking `testkit/kind`, and a
  `test-integration` Make target. The suite covers manager startup, Helm
  rendering, SSA deployment, ownership, GC, status, and finalizer cleanup.
- [x] T15 validation passed: v2 unit/race tests, integration-package compile,
  root-configured golangci-lint for the integration package, and a live Kind
  attempt through the Go provider. The live run explicitly skipped because
  Docker/Podman was unavailable.
- [x] T16 removed the obsolete nested framework module and migrated repository
  documentation and Makefile references; the root module remains as the
  explicitly retained v1 compatibility surface.
- [x] T22 added a reconciler-owned, factory-configurable condition manager
  without adding condition state to `pipeline.Request`.
- [x] T23 added OpenShift API-server TLS profile helpers, startup fallback, and
  a semantic-change watcher under `v2/pkg/kube/openshift/tls`.
- [x] T24 added explicit bootstrap cluster-distribution, OpenShift, OLM, and
  CRD discovery primitives with shared GVK constants; product profile
  composition remains caller-owned.
- [ ] T17–T20 review, validation, and closeout tasks executed.
- [x] T12 validation passed: `make -C v2 verify-fmt`, full v2 unit tests,
  full v2 race tests, `go vet ./...`, `go mod tidy -diff`, and focused
  controller/resource tests.
- [ ] The pinned `make -C v2 lint` check remains blocked by unavailable
  `proxy.golang.org` DNS access; rerun it when dependency network access is
  available.
- [ ] Final implementation validation passed.
