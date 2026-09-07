# T01 — Isolated Kind test engine

## Objective

Create the standalone Kind cluster engine required by later integration tests.

## Instructions

- Keep the engine in a separable package/module with its own dependency
  boundary.
- Do not import v1 framework helpers or v2 runtime packages.
- Provide cluster creation, readiness, kubeconfig/client access, cleanup,
  configurable naming/runtime settings, and deterministic failure reporting.
- Keep lifecycle ownership explicit and cleanup safe on partial startup.

## Verification

Unit-test option/default/error behavior. Add a smoke test that creates a Kind
cluster when the required external tooling is available; make unavailable
tooling skip explicitly rather than silently pass.

## Dependencies

None. Complete before the v2 integration suite and before other implementation
tasks that need a real cluster.
