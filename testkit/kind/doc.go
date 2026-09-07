// Package kind provides an isolated lifecycle wrapper for disposable Kind
// clusters used by integration tests.
//
// The package intentionally depends only on Kubernetes client libraries and
// the Kind Go provider library. It does not depend on any ODH framework or
// runtime package so it can be moved to a separate test-support repository.
package kind
