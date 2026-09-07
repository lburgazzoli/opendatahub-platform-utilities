# T08 — Action pipeline

## Objective

Implement immutable pipeline registration and before/main/after/cleanup
execution.

## Instructions

- Own `Request`, extensions, action registration, guards, optional validators,
  names, and registration options under `pkg/controller/pipeline`.
- Execute registrations in order, aggregate one `ActionError`, short-circuit
  only main actions, and keep cleanup separate.
- Keep concrete action packages out of pipeline implementation imports.

## Verification

Test ordering, duplicate/empty names, guard behavior, validator behavior,
phase-specific continuation, skipped actions, and cleanup semantics.

## Dependencies

T03, T04, T05, and T07.
