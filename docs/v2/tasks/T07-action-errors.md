# T07 — Action error contract

## Objective

Implement the canonical v2 `pkg/action.ActionError` semantics.

## Instructions

- Follow `docs/action-error-semantics.md` as the normative companion to
  `docs/v2/v2.md`.
- Preserve classifiers, precedence, joined/wrapped error inspection, explicit
  delays, status interpretation, events, and controller result mapping.
- Do not introduce an execution report or a second aggregation model.

## Verification

Cover plain, terminal, non-blocking, advisory, joined, wrapped, delayed,
phase-scoped, and cleanup outcomes with table-driven tests.

## Dependencies

T02. Can run in parallel with T03–T06.
