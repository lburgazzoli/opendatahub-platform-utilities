# T11 — Supporting actions

## Objective

Implement retained non-deploy/GC actions with the same `Run` plus thin
`Execute` model.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Add delete, requirements, workload, OpenShift, and release actions under
  `pkg/action` as defined by `v2.md`.
- Keep observation actions side-effect focused: return observations or update
  opt-in conditions, but do not persist status.
- Keep controller-specific rendering out of shared actions.

## Verification

Unit-test each action programmatically and through its adapter, including
missing inputs, API availability, condition access, and OpenShift-specific
behavior.

## Dependencies

T03, T04, T05, T06, T07, and T08. Can run in parallel with T09 and T10.
