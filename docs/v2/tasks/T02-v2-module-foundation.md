# T02 — V2 module foundation

## Objective

Establish the single `/v2` Go module and its build/architecture foundations.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Set the v2 module path and Go 1.26 toolchain policy.
- Add Makefile/CI coverage for tests, race tests, lint, tidy, formatting, and
  generated-code verification.
- Add architecture checks for the dependency direction in `docs/v2/v2.md`.
- Add the generated consumer-CR fixture required by the API contract.

## Verification

The new module builds independently, architecture checks fail on forbidden
imports, and generated-code verification is deterministic.

## Dependencies

T00 and T01.
