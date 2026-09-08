# T14 — Consumer migration

## Objective

Migrate examples and the integration-test consumer to the v2 API, moving the
reusable integration harness from `framework/testing` to `testkit/integration`.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Move `framework/testing` to `testkit/integration` and update its module path,
  imports, and documentation to consume v2 APIs.
- Add a `testkit/go.work` covering `v2`, `testkit/kind`, and
  `testkit/integration`; do not use local `replace` directives.
- Update examples and supported documentation to `/v2` imports.
- Keep `flakiness/` outside the migration.

## Verification

Build and test the migrated testkit module and examples from their workspaces.
Search for stale v1 imports, old request fields, the removed
`framework/testing` path, and local `replace` directives.

## Dependencies

T12 and T13.
