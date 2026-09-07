# T10 — Garbage-collection action

## Objective

Implement reusable GC and discovery behavior under `pkg/action/gc`.

## Instructions

- Consolidate root/framework GC behavior and retain RBAC-aware discovery,
  predicates, safelists, ownership checks, metadata-policy matching, metrics,
  and propagation policy.
- Make discovery a required constructor dependency with static and dynamic
  implementations.
- Keep dynamic discovery invalidation and concurrency behavior explicit.

## Verification

Test ownership safety, desired identity matching, policy composition, RBAC
filtering, static/dynamic discovery, invalidation, safelists, and deletion
behavior. Test `Run` and `Execute` parity.

## Dependencies

T05, T06, T07, and T08. Can run in parallel with T09 and T11.
