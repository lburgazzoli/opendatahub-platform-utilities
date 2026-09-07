# T16 — Legacy removal

## Objective

Remove duplicate implementations and obsolete module boundaries after v2
behavior is verified.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Delete the nested framework module and obsolete root/framework duplicates only
  after consumer migration and acceptance tests pass.
- Remove compatibility-only wrappers, stale aliases, and v1 imports that are
  outside the explicitly retained v1 modules.
- Preserve `flakiness/` as a separate module.

## Verification

Run repository-wide searches for forbidden v1 imports and duplicate package
trees, then run all module and generated-code checks.

## Dependencies

T14 and T15.
