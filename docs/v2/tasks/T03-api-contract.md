# T03 — V2 API contract

## Objective

Implement the type-only v2 `api` package and its contract validation.

## Instructions

- Consolidate the platform wire types, constants, reduced accessors,
  `PlatformProfile`, and optional status capabilities described by `v2.md`.
- Preserve required JSON tags, schema markers, and generated deepcopy behavior.
- Keep constructors and behavioral helpers out of `api`.

## Verification

Test required and optional accessor round-tripping, JSON compatibility,
deepcopy generation, and contract validation against consumer fixtures.

## Dependencies

T02. Can run in parallel with T04–T07.
