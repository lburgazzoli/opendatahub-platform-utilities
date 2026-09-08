# T21 — Standalone controller examples

## Objective

Add two complete, independently buildable controller-runtime examples that
render and deploy the same Helm chart: one using the v2 reconciler builder and
one using a plain `Reconcile` implementation.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Keep both examples under `v2/examples`, with separate Go modules and a
  workspace that references the local v2 module without a `replace` directive.
- Use controller-runtime project conventions: `cmd`, versioned API,
  `internal/controller`, generated deepcopy/CRD/RBAC output, and Makefiles.
- Use the v2 config package for startup options and the v2 manager/client
  wrappers, and configure the controller-runtime manager, client, and cache
  through normal controller-runtime options. Keep the chart, API, deploy
  options, and status-condition behavior equivalent between the two
  controllers; vary only builder versus plain reconciliation wiring.
- Do not use the Kind binary. If live-cluster coverage is needed, use the
  isolated Kind Go-library engine from the v2 test infrastructure.

## Verification

Run formatting, controller generation, vet, golangci-lint, unit tests, and
race tests for both example modules. Confirm that generated CRD/RBAC output is
current and that both controllers expose the same successful and failed status
conditions.

## Dependencies

T12 and T13. The examples are an independent consumer task and do not change
the v2 action or reconciler implementation.
