# V2 Consumer and Capability Audit

This audit is the T00 baseline for the redesign in [`v2.md`](v2.md). It
describes the checked-out repository only; external module-controller imports
must be rechecked before removing a public v1 symbol.

## Current module boundaries

| Module | Current role | V2 treatment |
| --- | --- | --- |
| Root module | Platform contract and low-level Kubernetes utilities | Merge into the `/v2` module |
| `framework/` | Reconciler, action pipeline, rendering/action integrations, and framework helpers | Merge into `/v2`, reshaping packages under `api` and `pkg` |
| `testkit/integration/` (migrated from `framework/testing/`) | Live-cluster integration harness | Keep outside the public v2 runtime; consume v2 APIs |
| `flakiness/` | CI artifact, runtime, quarantine, and Jira tooling | Remain a separate module and outside the v2 redesign |

The current root, framework, and framework-testing modules use Go 1.25.x.
V2 must establish one Go 1.26 module and remove the framework module boundary.

## Capability ownership today

| Capability | Current implementations | V2 owner |
| --- | --- | --- |
| Platform contract | `api/common`, `framework/api` | `api` |
| Contract validation | `api/common/validation` | `pkg/platform/validation` |
| Conditions | `pkg/controller/conditions`, `framework/controller/conditions` | `pkg/platform/condition` |
| Metadata protocols | `pkg/metadata`, `framework/metadata` | `pkg/platform/metadata` |
| Resource operations | `pkg/resources`, `framework/resources` | `pkg/kube/resources` |
| Ownership and singleton behavior | `pkg/cluster`, `framework/cluster`, webhook helpers | `pkg/kube/ownership`, `pkg/kube/singleton`, `pkg/kube/admission/singleton` |
| Apply/status writes | root resources, framework resources, `pkg/status` | `pkg/kube/resources` |
| Deployment | `pkg/deploy`, framework deploy action | `pkg/action/deploy` |
| Garbage collection | `pkg/controller/gc`, framework GC action | `pkg/action/gc` |
| Action errors | framework action error package | `pkg/action` |
| Pipeline/reconciler | framework controller packages plus root action helpers | `pkg/controller/pipeline`, `pkg/controller/reconciler` |
| Rendering | root/framework renderers and caches | direct manifest-kit use by module-owned actions |
| TLS | `pkg/cluster`/TLS-related integration | `pkg/kube/openshift/tls` |
| Test helpers | framework testing and matchers | isolated Kind engine plus migration consumer; no public v2 test-helper promise |

## Known migration risks

1. `api/common` and `framework/api` contain distinct Go types. V2 must choose
   one contract and add explicit compatibility/migration guidance rather than
   aliasing both implementations.
2. The root and framework deploy/GC implementations overlap but do not have
   identical policy surfaces. V2 must preserve production behavior while
   selecting one implementation and testing the merged behavior.
3. V1 request fields such as `SkipDeploy`, `SkipApply`, and `Generated` are
   intentionally removed by v2. Existing scenario docs and custom actions need
   semantic migration, not mechanical renaming.
4. V1 rendering packages are local implementations. V2 must verify actual
   consumer behavior before replacing them with direct manifest-kit usage.
5. `pkg/status` uses conflict-retry update semantics while v2 specifies SSA
   status application. This is a deliberate behavior change requiring focused
   migration tests.
6. Existing integration documentation pointed at `framework/testing`; the
   harness must remain outside the public v2 runtime and must not become a
   hidden runtime dependency.
7. The current repository contains stale v1-oriented documentation references.
   They should be updated only by the consumer-migration task, not during
   foundational package work.

## Initial task implications

- T01 must produce a standalone Kind engine before real-cluster tests are
  written, but runtime v2 packages must not import it.
- T02 must establish the module and architecture checks before duplicate
  implementations are consolidated.
- T03–T07 can proceed in parallel after the foundation, with package ownership
  following the v2 layout.
- T14 must move the integration harness to `testkit/integration`, migrate it to
  v2 APIs, and migrate examples and other test consumers after the public APIs
  stabilize.
- T16 must not remove v1 implementations until retained behavior and known
  consumer imports have been audited.

## Audit conclusion

The repository has clear implementation overlap for the capabilities listed in
`v2.md`; no capability is proposed for removal at T00. The high-risk areas are
the API-contract split, action/request semantics, status-write behavior,
renderer replacement, and test-harness isolation. These risks are covered by
dedicated later tasks and must remain visible during adversarial review.
