# T15 — Kind integration suite

## Objective

Add real-cluster integration coverage for the retained v2 behavior.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Use only the isolated Kind engine from T01 and public v2 APIs.
- Cover representative controller startup, reconcile, deploy, GC, ownership,
  status, cleanup, and renderer flows.
- Keep fixtures and cluster lifecycle isolated so the test engine can move to a
  separate repository without importing framework helpers.

## Verification

Run deterministic setup/teardown, assert meaningful Kubernetes state, and make
missing external Kind tooling an explicit skip. Ensure failures preserve logs
and cluster diagnostics where possible.

## Dependencies

T01, T12, and T14.
