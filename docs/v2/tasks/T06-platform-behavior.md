# T06 — Platform behavior

## Objective

Implement behavior over v2 API types without expanding the `api` package.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Consolidate condition CRUD/aggregation, release helpers, metadata policies,
  and runtime platform-contract validation under `pkg/platform`.
- Keep status computation and persistence out of action implementations.
- Preserve the v2 decision to defer active-condition/stale-condition behavior
  unless a concrete retained consumer requires it.

## Verification

Test condition severity/aggregation, release operations, metadata policy
composition, immutability, and optional accessor validation.

## Dependencies

T03 and T04. Can run in parallel with T05 and T07.
