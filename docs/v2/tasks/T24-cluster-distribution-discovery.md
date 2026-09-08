# T24 — Cluster and distribution discovery

## Objective

Add bootstrap-time cluster and distribution discovery for v2 without restoring
the removed root discovery result types.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Put generic Kubernetes detection and CRD checks under
  `v2/pkg/kube/cluster`.
- Put OpenShift facts under `v2/pkg/kube/openshift` and OLM reads under
  `v2/pkg/kube/olm`.
- Return the distribution-neutral `api.Distribution` from the generic cluster
  probe and compose `api.PlatformProfile` explicitly in the distribution
  integration; do not add discovery to the reconciler or pipeline request.
- Do not add a universal product detector or source-precedence policy; expose
  independent primitives so the consuming distribution integration owns that
  decision.
- Keep `NoMatch` and `NotFound` semantics distinct and reject malformed
  unstructured discovery data instead of treating it as absence.
- Preserve explicit `NotFound`/`NoMatch`/unexpected-error semantics, require
  an operator namespace for namespace-scoped distribution probes, and cover
  present, absent, and failing APIs with focused tests.

## Verification

Run the v2 race-tested unit suite, `go vet ./...`, and the configured linter
when dependency access is available.

## Dependencies

T16 legacy removal.
