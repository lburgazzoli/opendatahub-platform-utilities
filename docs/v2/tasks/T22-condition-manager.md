# T22 — Reconciler condition manager

## Objective

Add configurable reconciler-owned condition status processing without putting
a condition manager into `pipeline.Request`.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Keep action-owned condition writes on `api.ConditionsAccessor`.
- Wrap framework outcome marking and aggregation behind a reconciler-owned
  condition manager factory.
- Add focused tests for the default and custom manager paths.

## Verification

Run the v2 race-tested unit suite and `go vet ./...`.

## Dependencies

T12 controller integration.
