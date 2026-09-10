# T25 — Object-level requirements

## Objective

Extend `pkg/action/requirements` with object-level existence and absence
checks that preserve the existing requirements action contract.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

Keep the implementation compatible with the existing `RequireAPIs` and
`ForbidAPIs` actions. Do not create a separate `sanitycheck` package.

## Instructions

- Add programmatic and pipeline forms for requiring objects and forbidding
  objects.
- Support both exact object references and GVK-level checks for any object of
  that GVK. An empty object name means “any matching object”; a name identifies
  one object. Namespace filtering must remain explicit.
- Represent each target with its complete GVK and namespaced identity. Do not
  infer a kind from a partial or kind-only value.
- Keep API absence semantics compatible with requirements:
  - a forbidden object succeeds when its API is unavailable or the object is
    not found;
  - a required object reports the requirement as unsatisfied when its API or
    object is absent;
  - unexpected client, mapper, or list errors remain errors.
- Use dedicated condition types for object requirements, while preserving the
  existing condition accessor, observed-generation, terminal action-error, and
  pipeline adapter behavior.
- Reuse controller-runtime and Kubernetes API machinery for object lookup and
  list behavior. Do not add a second discovery or object identity abstraction.
- Keep constructors error-free, validate stable configuration at construction,
  and validate invocation dependencies in `Run` as required by `v2.md`.
- Preserve deterministic messages containing the full GVK and object identity.

## Verification

Add focused unit tests for both `Run` and `Execute` covering:

- exact required object present and absent;
- exact forbidden object present and absent;
- GVK-level required and forbidden checks;
- namespaced and cluster-scoped targets;
- missing CRD/API behavior;
- `NotFound`, `NoMatch`, mapper, list, and get errors;
- condition updates and observed generation;
- terminal action-error classification; and
- missing action, client, instance, and condition inputs.

Use the existing vanilla Gomega testing conventions and run the focused
requirements tests, race tests, formatting, vet, and lint checks.

## Dependencies

T11. This task may run in parallel with T12 and T13 because it owns only
`v2/pkg/action/requirements` and its tests.

T25 must complete before T19 final validation.
