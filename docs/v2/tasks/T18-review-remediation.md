# T18 — Review remediation

## Objective

Use `luna-high` to fix every accepted T17 finding.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Implement only accepted findings and their directly required regression
  tests.
- Do not broaden scope or silently reject findings to make validation pass.
- Update `findings.md` with the fix and verification evidence.

## Verification

Run each affected focused test, the relevant integration tests, and the
architecture checks. Return unresolved findings to T17 for re-review.

## Dependencies

T17.
