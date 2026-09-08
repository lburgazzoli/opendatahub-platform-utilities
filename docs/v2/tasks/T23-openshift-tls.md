# T23 — OpenShift TLS integration

## Objective

Add the v2 OpenShift API-server TLS profile helpers under
`v2/pkg/kube/openshift/tls`.

## Non-negotiable rules

Read and follow [`../development.md`](../development.md) before starting.

## Instructions

- Reuse the official OpenShift TLS profile types and crypto mappings.
- Support profile resolution, Go TLS configuration, API-server startup
  fallback, and a controller-runtime watcher for profile changes.
- Treat a missing OpenShift API as a stable Intermediate-profile fallback and
  transient API failures as a watchable fallback; return unexpected errors.
- Use semantic equality for watcher change detection and cover fallback,
  conversion, and watcher-facing behavior with focused tests.

## Verification

Run the v2 race-tested unit suite and `go vet ./...`.

## Dependencies

T16 legacy removal.
