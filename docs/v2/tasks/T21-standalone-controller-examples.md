# T21 — Standalone controller examples

## Objective

Add a complete, independently buildable controller-runtime example that renders
and deploys a Helm chart through the v2 reconciler builder. A plain
`Reconcile` comparison example is intentionally deferred until the framework
pipeline example is established.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Keep the example under `v2/examples` with its own Go module and a workspace
  that references the local v2 module without a `replace` directive. Keep
  integration tests inside that example module under `test/integration` and
  keep the isolated Kind engine as the only separate test-support module.
- Use controller-runtime project conventions: `cmd`, versioned API,
  `internal/controller`, generated deepcopy/CRD/RBAC output, and Makefiles.
- Use the v2 config package for startup options and the v2 manager/client
  wrappers, and configure the controller-runtime manager, client, and cache
  through normal controller-runtime options. Keep the chart, API, deploy
  options, and status-condition behavior aligned with the v2 proposal. Keep
  the chart under `config/chart` and make its path configurable.
- Do not use the Kind binary. If live-cluster coverage is needed, use the
  isolated Kind Go-library engine from the v2 test infrastructure.

## Verification

Run `make -C v2/examples all` to cover formatting, controller generation, vet,
golangci-lint, unit tests, and the in-module integration tests. The unit-only
path remains available through `make -C v2/examples test`, while
`make -C v2/examples test-integration` runs the Kind-backed test explicitly.
Confirm that generated CRD/RBAC output is current and that the controller
exposes the successful status condition through a real Kind-backed
reconciliation.

## Dependencies

T12 and T13. The examples are an independent consumer task and do not change
the v2 action or reconciler implementation.
