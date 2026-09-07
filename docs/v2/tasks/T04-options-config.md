# T04 — Options and configuration

## Objective

Implement immutable generic options and typed configuration source composition.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Add the dependency-free generic option primitives.
- Implement complete struct options and functional options with documented
  zero/nil, cloning, merge, and precedence semantics.
- Add defaults, mounted-file, and environment sources with the specified
  precedence; keep module schemas outside the shared package.

## Verification

Test equivalent struct/function options, cloning, precedence, invalid values,
and concurrent safe reuse of constructed configuration/actions.

## Dependencies

T02. Can run in parallel with T03 and T05–T07.
