# T09 — Deploy action

## Objective

Implement the reusable deploy action with programmatic and pipeline entry
points.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Consolidate root and framework deploy behavior under `pkg/action/deploy`.
- Keep `Run` independent of pipeline requests and keep `pipeline.go` limited to
  request mapping.
- Preserve normalization, metadata policy, ordering, merge strategies, cache,
  CRD handling, ownership rules, and continue-on-error behavior required by
  `v2.md`.

## Verification

Test programmatic/pipeline parity, invalid and duplicate identities, metadata
publication, merge/customizer behavior, caching, ordering, and SSA/patch paths.

## Dependencies

T05, T06, T07, and T08. Can run in parallel with T10 and T11.
