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
- Before committing each task, make the implementation and its tests readable:
  separate logical setup, execution, error-handling, synchronization, and
  assertion blocks with blank lines, and remove dense or misleading formatting.
  Keep an error-producing statement directly adjacent to its `if err != nil`
  check; place blank lines around the complete error-handling block instead.

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
- Keep Go code highly readable: separate guard clauses, independent
  computations, error checks, lock sections, and return statements with blank
  lines. Do not compress several logical operations into one dense block.
- Before implementing a method or function, check whether the same behavior
  already exists in this repository or in the Go standard library,
  controller-runtime, client-go, or Kubernetes APIs. Reuse existing behavior
  instead of recreating it.
- Deployment must preserve the namespace supplied by the caller; do not infer
  or default a desired resource namespace from the owner object.
- Generic deploy customizers are defaults. A caller-provided per-GVK
  customizer replaces the default; compose built-in behavior explicitly when
  a resource needs both core and resource-specific customization.
- `resources.Accessor` iteration yields borrowed objects. Actions must deep-copy
  objects before normalization or decoration and publish the replacement
  collection only after preparation succeeds. Use `Len` plus `All` when a
  private list needs capacity, and append iterator results instead of relying
  on caller-visible indexes.
- `resources.Accessor` stores `unstructured.Unstructured` values. Producers
  convert typed objects before publishing them to the accessor; deploy must
  consume the published unstructured values directly and must not add a
  typed-to-unstructured round trip.
- Prefer standard-library helpers over one-line wrappers, such as
  `slices.Clone` instead of a local slice-cloning helper.
- Deploy looks up the current object once per desired resource. Reuse that
  object for skip checks, customizers, and cache lookup; do not add another
  lookup only to record a cache entry. Take the cache fingerprint after
  ownership and customizer changes, but before SSA mutates the desired object.
  `resources.Apply` updates the desired object, which is then used as the
  deployed cache object.
- Low-level apply helpers must forward caller-supplied apply options without
  injecting ownership policy. Callers that require forced SSA ownership pass
  `client.ForceOwnership` explicitly.
- A public cache type must have an exported constructor. Do not leave public
  option fields or constructors that are not consumed by the implementation.
- The managed-resource opt-out annotation is presence-based in v2: any
  existing object carrying the configured key is skipped. Keep this behavior
  explicit in comments and tests.
- Deployment resource merging intentionally preserves the legacy behavior,
  even when that behavior prevents explicit desired resource changes. Treat it
  as compatibility policy and do not “correct” it during cleanup work.
- Validate immutable action configuration once during construction and cache the
  result; each `Run` must still check that cached result before validating
  invocation inputs or performing I/O.
- Benchmark changes at the operation boundary that matters. Keep direct typed
  versus unstructured apply benchmarks separate from full fake-client runs so
  client/server-emulation overhead is not mistaken for deploy-action overhead.

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
