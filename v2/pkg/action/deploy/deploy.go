// Package deploy applies a desired Kubernetes resource collection.
package deploy

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrRunInputRequired  = errors.New("deploy run input is required")
	ErrDuplicateIdentity = errors.New("duplicate resource identity")
	ErrActionRequired    = errors.New("deploy action is required")
	ErrMetadataPolicy    = errors.New("deploy metadata policy is required")
	ErrFieldOwner        = errors.New("deploy field owner is required")
)

// CustomizerFunc can modify an object immediately before it is applied. The
// live object is nil when it does not exist.
type CustomizerFunc func(
	ctx context.Context,
	kubernetesClient client.Client,
	options Options,
	desired *unstructured.Unstructured,
	existing *unstructured.Unstructured,
) error

// Apply invokes the customizer. A nil customizer intentionally does nothing.
func (f CustomizerFunc) Apply(
	ctx context.Context,
	kubernetesClient client.Client,
	options Options,
	desired *unstructured.Unstructured,
	existing *unstructured.Unstructured,
) error {
	if f == nil {
		return nil
	}

	return f(ctx, kubernetesClient, options, desired, existing)
}

// FieldOwnerFunc resolves the SSA field manager for a run owner.
type FieldOwnerFunc func(client.Object) string

// Action deploys resources using an immutable configuration snapshot.
type Action struct {
	options       Options
	validationErr error
	cache         *Cache
	validated     bool
}

// New creates an immediately usable deploy action.
func New(values ...Option) *Action {
	options := defaultOptions()
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	action := &Action{
		options: options,
		cache:   newCache(options.Cache),
	}
	action.validationErr = action.Validate()
	action.validated = true
	return action
}

// Name returns the default pipeline registration name.
func (a *Action) Name() string { return "deploy" }
