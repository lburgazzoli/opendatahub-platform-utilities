# V2 Development Rules

These rules are mandatory for every implementation task in the v2 plan. The
task documents link here so an agent starting with clean context has the same
constraints as the original implementation.

## Source of truth and scope

- Read `docs/v2/v2.md` before changing code; it is the authoritative design.
- Read the task's related normative documents, especially
  `docs/action-error-semantics.md` for action errors.
- Do not add compatibility behavior, duplicate abstractions, or dependencies
  merely to make a local test pass. Record unresolved ambiguity in
  `docs/v2/findings.md`.
- Complete and validate one task before committing it. Each completed task
  gets its own focused commit; do not mix unrelated task work.

## Go and file layout

- Use the repository Makefile for Go validation whenever the module provides a
  target; use Go 1.26 language and library conventions.
- Apply the `use-modern-go` skill when writing or changing Go. Run its Modern Go
  Guidelines CLI for the target file and follow the complete applicable list.
- Functional-option APIs must define an `Option` interface. The complete
  `Options` struct must implement that same interface as functional options;
  test both forms when options are introduced.
- Put option declarations and option constructors in descriptively prefixed
  `*_options.go` files. Put supporting helpers, internal adapters, and test
  doubles in matching `*_support.go` files. Never create generic
  `options.go` or `support.go` files for a feature.
- Preserve dependency direction and keep package boundaries explicit. Avoid
  importing framework helpers into isolated test infrastructure.
- When classifying mutually exclusive error outcomes, prefer a `switch` with
  one case per classification over a sequence of related `if` statements.
- Do not group function parameters by type in new or modified declarations;
  write each parameter explicitly (for example, use `desired *T, current *T`
  rather than `desired, current *T`).
- Add a helper function only when it has a meaningful reuse or abstraction
  boundary; inline one-off calls and values when a helper would merely hide a
  single operation.
- Deployment must preserve the namespace supplied by the caller; do not infer
  or default a desired resource namespace from the owner object.

## Tests and integration infrastructure

- Use the standard `testing` package with vanilla Gomega assertions via
  `NewWithT(t)`; use Gomega matchers for errors (`MatchError`, `HaveOccurred`,
  `Succeed`, and related matchers), not manual error-string assertions.
- Use `testify/mock` for interaction-based mocks when a mock is needed; keep
  mocks narrow and assert their expectations with the test's Gomega instance.
- Use `t.Context()` rather than `context.Background()` in tests.
- Add focused unit tests with the implementation and integration tests when
  behavior depends on Kubernetes API semantics. Keep tests parallel-safe.
- Kind integration tests must use the Kind Go library/provider API, never the
  `kind` binary or subprocesses. The isolated engine must be movable to another
  repository without importing this repository's framework helpers.
- Missing external container tooling may produce an explicit skip only in
  environment-dependent integration tests; unit tests must remain deterministic.

## Review and handoff

- Run formatting, unit tests, race tests where the module supports them, vet,
  lint, and tidy checks appropriate to the changed module.
- Treat adversarial review findings as implementation work: record evidence,
  fix accepted findings, add regression tests, and rerun the affected checks.
- Update `docs/v2/plan.md` only with evidence from completed task commits and
  validation commands.
