# T13 — Renderer integration

## Objective

Replace local renderer implementations with direct manifest-kit integration.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Use manifest-kit Helm, Kustomize, and Go-template renderers directly.
- Keep rendering actions controller-owned; convert successful output into the
  shared resource accessor explicitly.
- Do not mirror upstream renderer APIs or add a generic shared render action or
  render cache.

## Verification

Test controller-owned render actions, value propagation, output publication,
cache configuration, render failure behavior, and composition of multiple
renderer types.

## Dependencies

T02, T04, T05, and T08. Can run in parallel with T09–T12.
