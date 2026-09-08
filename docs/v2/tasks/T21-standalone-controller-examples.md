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
  that references the local v2 module without a `replace` directive. Keep the
  isolated integration-test module in the same workspace and do not add local
  pseudo-version requirements for workspace modules.
- Use controller-runtime project conventions: `cmd`, versioned API,
  `internal/controller`, generated deepcopy/CRD/RBAC output, and Makefiles.
- Use the v2 config package for startup options and the v2 manager/client
  wrappers, and configure the controller-runtime manager, client, and cache
  through normal controller-runtime options. Keep the chart, API, deploy
  options, and status-condition behavior aligned with the v2 proposal.
- Do not use the Kind binary. If live-cluster coverage is needed, use the
  isolated Kind Go-library engine from the v2 test infrastructure.

## Verification

Run formatting, controller generation, vet, golangci-lint, unit tests, and
race tests for the example and its isolated integration module. Confirm that
generated CRD/RBAC output is current and that the controller exposes the
successful status condition through a real Kind-backed reconciliation.

## Dependencies

T12 and T13. The examples are an independent consumer task and do not change
the v2 action or reconciler implementation.
