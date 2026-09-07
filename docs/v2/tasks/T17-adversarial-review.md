# T17 — Adversarial review

## Objective

Have a fresh `sol-high` agent challenge the completed implementation and task
pack.

## Instructions

Review against `docs/v2/v2.md`, `docs/action-error-semantics.md`, `AGENTS.md`,
and the acceptance tests. Focus on dependency violations, API drift, missing
capability retention, unsafe cleanup, race conditions, weak tests, stale docs,
and Kind-engine coupling. Record each finding with severity, evidence, and a
precise remediation task in `docs/v2/findings.md`.

## Verification

Review is complete only when every finding is classified as accepted,
rejected with evidence, or deferred with an explicit owner and reason.

## Dependencies

T16.
