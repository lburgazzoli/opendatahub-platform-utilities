# T14 — Consumer migration

## Objective

Migrate examples and the integration-test consumer to the v2 API.

## Instructions

- Update examples and supported documentation to `/v2` imports.
- Rewrite `framework/testing` only as needed to consume v2; do not preserve its
  old public API or module shape as a v2 compatibility target.
- Keep `flakiness/` outside the migration.

## Verification

Build and test migrated examples and the testing consumer. Search for stale v1
imports, old request fields, and removed package paths.

## Dependencies

T12 and T13.
