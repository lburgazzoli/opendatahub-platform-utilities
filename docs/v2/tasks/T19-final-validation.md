# T19 — Final validation

## Objective

Run the complete deterministic validation suite before closeout.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Run unit tests, race tests, lint, formatting, tidy checks, generated-code
  checks, architecture checks, and Kind integration tests.
- Validate the examples with `make -C v2/examples test` for the unit-only path
  and `make -C v2/examples test-integration` for the in-module Kind test; the
  aggregate `make -C v2/examples all` target runs both without making the unit
  test target require a container runtime.
- Confirm the v2 module builds independently and no forbidden v1 imports or
  stale documentation links remain.
- Record command results and any environment-dependent skips in `plan.md`.

## Verification

All required checks pass, or every environment limitation is explicitly
recorded and independently reviewed.

## Dependencies

T18.
