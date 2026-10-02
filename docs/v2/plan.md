# V2 Implementation Plan

Status: **complete; T00–T25 complete**

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
                      -> T25 object requirements
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
T18 + T25 -> T19 final validation
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
- [x] All task files exist and have objective, verification, and dependency
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
- [x] SSA ownership policy is explicit at callers, and the per-resource
  deployment helper is colocated with run orchestration in `210a84b`.
- [x] T10 GC action implemented with policy-aware desired-set cleanup,
  authorization filtering, static/dynamic discovery, and focused tests in
  `8d9639e`.
- [x] T11 supporting actions implemented with safe selectors, isolated
  pipeline adapters, OpenShift ImageStream parsing, release discovery, and
  focused tests in `5bdd036`.
- [x] T12 controller integration implemented with handlers, predicates,
  reconciler lifecycle, status/finalizer handling, dynamic ownership, and
  GVK resource helpers in `798bada`.
- [x] T12 watch-parity follow-up added typed and GVK watch registration,
  conditional `When`/`Dynamic` watches, and legacy owned, unmanaged, and CRD
  dynamic-watch routing with focused tests.
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
- [x] T22 added reconciler-owned condition processing without adding condition
  state to `pipeline.Request`; aggregation uses explicit controller-configured
  dependent condition types and always includes `ProvisioningSucceeded`.
- [x] T23 added OpenShift API-server TLS profile helpers, startup fallback, and
  a semantic-change watcher under `v2/pkg/kube/openshift/tls`.
- [x] T24 added explicit bootstrap cluster-distribution, OpenShift, OLM, and
  CRD discovery primitives with shared GVK constants; product profile
  composition remains caller-owned.
- [x] T17 completed the independent `sol-high` adversarial review; findings
  are recorded in [`findings.md`](findings.md) and assigned to T18.
- [x] T18 accepted finding remediation and regression coverage completed;
  F-012 also added architecture checks for package placement and dependency
  boundaries.
- [x] T25 added object-level `RequireObjects` and `ForbidObjects` actions with
  exact and GVK-level checks, condition updates, terminal errors, and pipeline
  adapters. Focused and full race-tested validation passed.
- [x] T19 final validation completed. The v2 race suite, v2 integration
  target, standalone `GOWORK=off go build ./...`, `go mod verify`, vet,
  formatting, architecture checks, Kind testkit, testkit integration, Helm
  example unit tests, Helm example integration tests, and root formatting and
  test targets were run. The root test target reached every module except the
  legacy `flakiness` module, which requires the unavailable Go 1.25.8
  toolchain.
- [x] T20 closeout completed. The master proposal remains authoritative at
  [`v2.md`](v2.md), all accepted review findings are fixed or explicitly
  classified as resolved/rejected in [`findings.md`](findings.md), and the
  final validation limitations are recorded above.
- [x] T12 validation passed: `make -C v2 verify-fmt`, full v2 unit tests,
  full v2 race tests, `go vet ./...`, `go mod tidy -diff`, and focused
  controller/resource tests.
- [x] The pinned lint, formatter, and controller-generator downloads remain
  explicitly environment-blocked by unavailable `proxy.golang.org` DNS
  access. The example aggregate target stops at the same formatter download;
  its unit and integration targets pass independently. A locally installed
  golangci-lint v2.13.2 was not treated as a substitute for the pinned
  v2.12.2 tool and reports existing baseline findings in addition to any
  validation output.
- [x] Final implementation validation passed with the environment-dependent
  skips recorded above. The standalone v2 build also required and now has the
  OpenShift module content hashes in `v2/go.sum`.

## Post-closeout deploy update

- [x] Deploy now enables its process-local cache by default, with an explicit
  `WithCache(false)` option and equivalent complete-struct configuration. Per-run
  labels and annotations override constructor metadata before policy stamping
  and cache fingerprinting. `RunOptions.Merge` and `RunOptions.Validate` own
  invocation option composition and validation, and every run field has a
  functional option.
- [x] Focused tests cover the default cache, cache opt-out, changing per-run
  metadata, metadata-policy precedence, map ownership, and option composition.
  `make -C v2 test`, `make -C v2 vet`, and `make -C v2 verify-fmt` passed with a
  writable Go build cache. `make -C v2 test-integration` passed with its live
  Kind test skipped because Docker was unavailable. The pinned formatter and
  linter remain blocked by unavailable `proxy.golang.org` DNS access; local
  `gofmt` formatted the changed Go files.
- [x] The deploy cache option now takes an explicit enabled boolean:
  `WithCache(false)` disables the default and `WithCache(true, settings)`
  enables it with optional TTL settings. Focused option tests and the v2 race
  suite, vet, and formatting checks passed; the pinned linter remained
  unavailable because `proxy.golang.org` DNS resolution failed.
- [x] Removed v2 Deployment probe carryover. Desired liveness, readiness, and
  startup probes now pass through the Deployment customizer unchanged; live
  replica and container-resource merging remains in place. A focused merge
  regression test and the v2 race suite, vet, and formatting checks passed.
- [x] Renamed the per-resource deployment helper to `deployResource` to describe
  its lookup, skip, customization, cache, and apply phases. The v2 race suite,
  vet, and formatting checks passed.
- [x] Reused `resources.SetLabels` and `resources.SetAnnotations` in the default
  metadata policy. The resource helpers now copy existing maps before merging,
  preserving the policy's metadata map ownership behavior. Regression tests,
  the v2 race suite, vet, and formatting checks passed. The pinned formatter
  and linter remained unavailable because `proxy.golang.org` DNS failed; local
  `gofmt` formatted the changed Go files.
- [x] The default policy now uses `resources.HasAnnotation` to match each
  owner-derived annotation, requiring keys to exist even when the expected
  owner value is empty. The v2 race suite, vet, and formatting checks passed;
  the pinned formatter and linter remained blocked by `proxy.golang.org` DNS.
- [x] Deploy now sorts before its single run loop, decorates each resource in
  that loop, accepts duplicate identities, and uses `deploy` for per-resource
  apply. The separate preparation pass, identity calculation, and preparation
  benchmark were removed. The v2 race suite, vet, and formatting checks passed;
  the pinned formatter and linter remained blocked by `proxy.golang.org` DNS.
- [x] Deploy now publishes the sorted collection before its run loop and uses
  `Resources.All()` to decorate each owned object in place. Caller-owned copies
  are the caller's responsibility; the extra copy and deferred publication were
  removed. The v2 race suite, vet, and formatting checks passed; the pinned
  formatter and linter remained blocked by `proxy.golang.org` DNS.
- [x] Moved the resource-list sorting function type to `pkg/kube/resources` so
  deploy options use the collection's canonical `resources.SortFunc`. The v2
  race suite, vet, and formatting checks passed; the pinned formatter and
  linter remained blocked by `proxy.golang.org` DNS.
- [x] Retried the pinned v2 formatter and linter outside the sandbox. The
  formatter passed without changing files. The linter ran and reported 93
  findings in the current codebase; its download is no longer blocked.
- [x] Added in-place sorting to `resources.Accessor` and `resources.Collection`,
  allowing deploy to order the collection without a `Get`/`Set` round trip. The
  v2 race suite, vet, formatting checks, and pinned formatter passed. The
  pinned linter ran and reported 94 current-codebase findings.
- [x] Added `resources.Identity.String()` using the existing
  `namespace/name GVK` diagnostic format, including cluster-scoped identities.
  The v2 race suite and vet passed. The changed Go files were formatted;
  repository formatting is held by a separate in-progress deploy edit. The
  pinned linter reported no findings in the identity code.
- [x] Changed `resources.Identity.String()` to a
  `group/version/kind/namespace/name` path, omitting the namespace segment
  for cluster-scoped resources. The v2 race suite and vet passed; the changed
  Go files are formatted. The pinned linter reported no findings in the
  identity files, with 98 existing findings elsewhere.
- [x] Included `resources.Identity` in deploy `Run` errors so failures name
  the exact resource, and covered both stop and continue-on-error behavior.
  The v2 race suite, vet, and formatter passed. The pinned linter still reports
  94 findings in the current tree.
- [x] Reworked deploy `Run` to apply labels before annotations and aggregate
  resource errors consistently while preserving the stop-on-error option.
  The v2 race suite, vet, and formatting checks passed; the pinned linter
  retained its 94 existing findings.

## Post-closeout platform modules example

- [x] The common `api.Status` now implements `ConditionsAccessor`; condition
  producers and reconciler status processing use `GetStatus()`. Reconciliation
  explicitly requeues after adding a finalizer because the primary predicate
  ignores finalizer-only updates. `make -C v2 test`, `vet`, and `fmt` passed.
  The pinned root v2 linter still reports its existing findings.
- [x] The Kind Go engine can collect logs on startup or test failure. Its race
  tests and pinned linter passed with zero findings.
- [x] Added the multi-group Platform, Serving, Kserve, and AI Gateway example.
  YAML module definitions supply the CRD identity and controller chart values;
  one chart template derives each module CRD. All controllers use v2
  `reconciler.For`, and Serving watches module GVKs after their CRDs appear.
  The module controller uses `resources.GetAnnotation` to simulate failures.
- [x] The example's generated APIs and reference CRDs, race tests, vet,
  formatter, and pinned linter passed. The complete Podman Kind suite passed
  both Platform/module and Serving flows in one process, including dynamic
  CRD installation, readiness, simulated failure recovery, and module removal.
- [x] Replaced the example's command-selection switch with Cobra subcommands
  for `run controller platform|serving` and `run module kserve|aigateway`.
  The module subcommands pass typed API objects to the shared controller.
  The example's formatter, race tests, linter, and build passed; CLI help
  paths were checked directly.
- [x] Changed the example's module descriptors to the deployer's
  `PlatformModuleConfig` YAML shape. Each module now owns an umbrella chart
  with conditional `module` and `projections` children. The module child
  installs its generated CRD and controller, while Serving renders spec and
  status apply objects through each module's projections child. The complete
  descriptor spec is passed under `.Values.module` for workload renders.
  `make manifests` generates module CRDs directly into those charts. The
  example's formatter, race tests, linter, build, e2e compilation, and both
  Podman Kind integration tests passed.
- [x] Split the example's Platform and PlatformModule controllers into separate
  packages. Each PlatformModule now deploys into `opendatahub-<name>-system`,
  records all rendered resources in status, prunes retired chart resources,
  and explicitly removes recorded controller resources on deletion after its
  configured module CR is gone. Namespace and CRD references stay in status
  and those resources are never pruned or deleted. Generated API artifacts,
  race tests, vet, e2e compilation, the pinned formatter and linter, and both
  Podman Kind integration tests passed.
- [x] Consolidated PlatformModule chart tests, enforced the module name match
  in its CRD, and decoded module specs into chart values with mapstructure
  tags. PlatformModule resource inventory checks now use Kubernetes sets;
  status inventory is deduplicated and sorted while retaining past Namespace
  and CRD references. Cleanup and inventory errors identify the failing step.
  The example's formatter, race tests, vet, e2e compilation, pinned linter,
  and both Podman Kind integration tests passed.
- [x] Replaced string-keyed PlatformModule chart value assembly with exported
  `ChartValues`, `ModuleValues`, and `ProjectionValues` types.
  Mapstructure converts the typed payload at the renderer boundary.
  The exact value shape, chart rendering, formatter, race tests, vet, e2e
  compilation, pinned linter, and both Podman Kind scenarios passed.
- [x] Made chart value conversion a standalone `ToValues(any)` function so
  conversion remains independent of `ChartValues`. The example's race tests,
  vet, formatter, e2e compilation, and pinned linter passed.
- [x] Made recorded-resource deletion a `Controller` method backed by its
  write client. The example's formatter, race tests, vet, e2e compilation,
  pinned linter, and both Podman Kind scenarios passed.
- [x] Added `gomega-matchers@main` to the example tests and used its Kubernetes
  getter and jq matchers for Serving status polling. The example's formatter,
  race tests, vet, e2e compilation, pinned linter, and both Podman Kind
  scenarios passed.
