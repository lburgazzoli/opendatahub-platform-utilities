# T22 — Reconciler condition processing

## Objective

Add configurable reconciler-owned condition status processing without putting
a condition manager into `pipeline.Request` or requiring a replaceable manager
factory for the default policy.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Keep action-owned condition writes on `api.ConditionsAccessor`.
- Keep framework outcome marking explicit and aggregate only the dependent
  condition types configured by the controller. Always include
  `ProvisioningSucceeded`.
- Add focused tests for the default and explicitly configured dependent-type
  paths.

## Verification

Run the v2 race-tested unit suite and `go vet ./...`.

## Dependencies

T12 controller integration.
