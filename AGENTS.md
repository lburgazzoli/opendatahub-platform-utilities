# ODH Platform Utilities — Agent Guide

`odh-platform-utilities` is a shared Go library for Open Data Hub module
controllers. New controller development targets the consolidated v2 module;
the root module remains as a v1 compatibility surface.

## Repository structure

| Module | Path | Purpose |
| --- | --- | --- |
| `github.com/opendatahub-io/odh-platform-utilities` | `/` | Retained v1 utilities |
| `github.com/opendatahub-io/odh-platform-utilities/v2` | `v2/` | Current platform contract and controller framework |
| `github.com/opendatahub-io/odh-platform-utilities/testkit/kind` | `testkit/kind/` | Kind Go-library engine |
| `github.com/opendatahub-io/odh-platform-utilities/testkit/integration` | `testkit/integration/` | Live-cluster integration harness |
| `github.com/opendatahub-io/odh-platform-utilities/flakiness` | `flakiness/` | CI flakiness tooling |

The old `framework/` module has been removed. Local development across v2 and
testkit modules uses checked-in `go.work` files, never `replace` directives.

## V2 layout

```text
v2/
  api/                         Platform contract and accessors
  pkg/action/                  Programmatic actions and pipeline adapters
  pkg/controller/pipeline/     Phased action pipeline
  pkg/controller/reconciler/   Controller-runtime lifecycle and status
  pkg/kube/                    Kubernetes resources and ownership helpers
  pkg/platform/                Conditions, metadata, release, validation
  test/fixture/                Generated consumer and renderer fixtures
  test/integration/             Kind-backed integration suite
```

Read [`docs/v2/v2.md`](docs/v2/v2.md) as the master design and
[`docs/v2/development.md`](docs/v2/development.md) as the non-negotiable
development rules before changing v2 code.

## Development rules

- Search the repository and standard Kubernetes/controller-runtime APIs before
  adding helpers or constants.
- Use functional options for optional configuration. Complete `Options`
  structs implement their package `Option` interfaces.
- Keep `_options.go`, `_support.go`, and action-prefixed file names consistent.
- Keep functions and tests highly readable: one logical block per paragraph,
  blank lines between phases, no grouped typed arguments, and no unnecessary
  one-use wrappers.
- Use `t.Context()` in tests, vanilla Gomega assertions and error matchers, and
  `testify/mock` only for interaction-based mocks.
- Use `new(value)` for pointer values where it improves clarity and use GVK
  constants/variables instead of repeated API-version/kind literals.
- Handle mutually exclusive Kubernetes errors in explicit `switch` cases, one
  case per line with its own body. Propagate malformed fields and unexpected
  API errors.
- Use the Kind Go library/provider API for cluster tests. Never invoke the
  `kind` binary or a subprocess.
- Run formatting and linting with the repository Makefile and root
  `.golangci.yml`; use `golangci-lint` for formatting and linting.
- Use the `use-modern-go` skill whenever writing or modifying Go code.
- After every task, make the changed code and tests readable before committing.

## Testing and commits

Use parallel-safe table-driven unit tests where appropriate. Integration tests
must have deterministic setup/teardown, meaningful Kubernetes assertions, and
an explicit skip when external container tooling is unavailable.

Commit each completed task separately. Record task completion and validation in
[`docs/v2/plan.md`](docs/v2/plan.md). Adversarial review uses `sol-high` and
accepted findings are fixed with `luna-high` before final validation.

## Commands

```bash
make -C v2 test
make -C v2 test-integration
make -C v2 lint
make -C testkit/kind test
make -C testkit/integration test
make -C flakiness test
```
