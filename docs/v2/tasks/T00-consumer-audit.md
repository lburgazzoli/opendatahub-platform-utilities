# T00 — Consumer and behavior audit

## Objective

Inventory v1 consumers, exported symbols, duplicate implementations, and
production capabilities before v2 code is created.

## Instructions

- Read `docs/v2/v2.md` and inspect all current Go modules and examples.
- Classify each retained capability as reshape, consolidate, internalize, or
  compatibility-only.
- Record known external-consumer risks and unresolved decisions in
  `docs/v2/findings.md` only when necessary.

## Verification

Provide a checked inventory and a migration-risk summary. Do not change runtime
code.

## Dependencies

None. This is a prerequisite for all implementation tasks.
