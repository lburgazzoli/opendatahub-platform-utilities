# T12 — Controller integration

## Objective

Implement handlers, predicates, reconciler lifecycle, status processing,
finalizers, cleanup, and dynamic ownership.

## Instructions

- Implement cohesive public handlers/predicates and the private reconciler
  lifecycle under `pkg/controller`.
- Load the primary object directly from the reconcile request using a
  deep-copyable prototype; keep singleton routing in handlers.
- Implement before/main/after execution, optional status accessors, SSA status
  apply, events, result interpretation, cleanup deadlines, and framework-owned
  dynamic-watch synchronization exactly as specified by `v2.md`.

## Verification

Test build validation, object loading, finalizer paths, status projection,
observed generation, action errors/results, cleanup timeout behavior, handlers,
predicates, and dynamic ownership synchronization.

## Dependencies

T03, T05, T06, T07, T08, T09, and T10.
