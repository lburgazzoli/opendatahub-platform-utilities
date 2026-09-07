// Package deletion contains an explicit bulk resource deletion action.
package deletion

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrActionRequired = errors.New("delete action is required")
	ErrTypesRequired  = errors.New("delete resource types are required")
	ErrTypeInvalid    = errors.New("delete resource type is invalid")
	ErrSelector       = errors.New("delete selector or explicit delete-all opt-in is required")
	ErrClientRequired = errors.New("delete client is required")
	ErrNamespace      = errors.New("delete namespace is required for namespaced resources")
)

// Result reports the outcome of a bulk deletion request. DeleteAllOf does not
// expose the number of objects removed, so the result is intentionally empty.
type Result struct{}

// Action deletes all resources matching the configured types and labels.
//
//nolint:govet // field order follows the public action policy grouping.
type Action struct {
	types         []client.Object
	labels        map[string]string
	namespace     string
	deleteAll     bool
	validationErr error
	validated     bool
}

// New creates an action for the supplied resource prototypes.
func New(types []client.Object, values ...Option) *Action {
	options := Options{}
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	clonedTypes, err := cloneTypes(types)
	action := &Action{
		types:     clonedTypes,
		labels:    maps.Clone(options.Labels),
		namespace: options.Namespace,
		deleteAll: options.DeleteAll,
	}
	if err != nil {
		action.validationErr = err
		action.validated = true

		return action
	}

	action.validationErr = action.Validate()
	action.validated = true

	return action
}

// Validate checks stable action configuration.
func (a *Action) Validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.validated {
		return a.validationErr
	}
	if len(a.types) == 0 {
		return ErrTypesRequired
	}
	if !a.deleteAll && len(a.labels) == 0 {
		return ErrSelector
	}

	return nil
}

// Run deletes matching resources.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Result, error) {
	err := a.Validate()
	if err != nil {
		return Result{}, err
	}

	options, err := resolveRunOptions(values...)
	if err != nil {
		return Result{}, err
	}
	if options.Namespace == "" {
		options.Namespace = a.namespace
	}

	for _, prototype := range a.types {
		namespaced, err := options.Client.IsObjectNamespaced(prototype)
		if err != nil {
			return Result{}, fmt.Errorf("resolve namespace for %s: %w", prototype.GetObjectKind().GroupVersionKind(), err)
		}

		deleteOptions := make([]client.DeleteAllOfOption, 0, 2)
		if len(a.labels) > 0 {
			deleteOptions = append(deleteOptions, client.MatchingLabels(a.labels))
		}
		if namespaced {
			if options.Namespace == "" {
				return Result{}, ErrNamespace
			}
			deleteOptions = append(deleteOptions, client.InNamespace(options.Namespace))
		}

		err = options.Client.DeleteAllOf(ctx, prototype, deleteOptions...)
		if err != nil {
			return Result{}, fmt.Errorf("delete %s resources: %w", prototype.GetObjectKind().GroupVersionKind(), err)
		}
	}

	return Result{}, nil
}
