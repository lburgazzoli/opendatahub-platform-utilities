# V2 Implementation Plan

Status: **implementation in progress; T00–T09 complete**

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
T01 + T12 + T14 -> T15 Kind integration suite
all implementation tasks -> T16 legacy removal
T16 -> T17 adversarial review (sol-high)
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
- [x] All 21 task files exist and have objective, verification, and dependency
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
- [ ] T10–T20 implementation and validation tasks executed.
- [ ] Final implementation validation passed.
