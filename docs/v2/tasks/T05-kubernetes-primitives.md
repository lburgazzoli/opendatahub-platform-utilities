# T05 — Kubernetes primitives

## Objective

Consolidate v2 Kubernetes resource, ownership, singleton, admission, metadata,
and server-side-apply primitives.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Implement `resources.Accessor` and the default collection semantics from
  `v2.md`.
- Consolidate decoding, identity, cloning, metadata transforms, ownership,
  singleton lookup, singleton admission, GVK constants, `Apply`, and
  `ApplyStatus`.
- Keep package boundaries under `pkg/kube`; do not import actions or pipeline.

## Verification

Test collection copy/write behavior, identity validation, ownership safety,
singleton behavior, metadata transformations, SSA conversion, and status apply.

## Dependencies

T02. Can run in parallel with T03, T04, T06, and T07.
