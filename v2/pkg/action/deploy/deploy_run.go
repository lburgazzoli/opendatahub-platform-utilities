package deploy

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

// Result reports the number of resources applied and skipped by a run.
type Result struct {
	Applied int
	Skipped int
}

// Validate checks stable action configuration.
func (a *Action) Validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.validated {
		return a.validationErr
	}
	if a.options.MetadataPolicy == nil {
		return ErrMetadataPolicy
	}
	if a.options.FieldOwner == nil {
		return ErrFieldOwner
	}
	return nil
}

// Run normalizes, decorates, and applies the desired resources.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Result, error) {
	err := a.Validate()
	if err != nil {
		return Result{}, err
	}
	runOptions, err := resolveRunOptions(values...)
	if err != nil {
		return Result{}, err
	}
	if a.cache != nil {
		a.cache.Sync()
	}

	objects, err := a.prepare(runOptions)
	if err != nil {
		return Result{}, err
	}

	result := Result{}
	var runErrors []error
	for _, object := range objects {
		applied, err := a.deployOne(ctx, runOptions, object)
		switch {
		case err != nil:
			wrapped := fmt.Errorf("deploy %s: %w", object.GetObjectKind().GroupVersionKind(), err)
			if !a.options.ContinueOnError {
				return result, wrapped
			}
			runErrors = append(runErrors, wrapped)
			continue
		case applied:
			result.Applied++
		default:
			result.Skipped++
		}
	}

	return result, errors.Join(runErrors...)
}

func (a *Action) prepare(values RunOptions) (resources.List, error) {
	seen := sets.New[resources.Identity]()

	objects := values.Resources.Get()
	for index, object := range objects {
		_, err := resources.EnsureGroupVersionKind(values.Client.Scheme(), object)
		if err != nil {
			return nil, fmt.Errorf("normalize resource %d: %w", index, err)
		}
		resources.SetLabels(object, a.options.Labels)
		resources.SetAnnotations(object, a.options.Annotations)
		err = a.options.MetadataPolicy.Apply(object, values.Owner)
		if err != nil {
			return nil, fmt.Errorf("decorate %s/%s: %w", object.GetNamespace(), object.GetName(), err)
		}
		identity, err := resources.IdentityOf(object, values.Client.Scheme())
		if err != nil {
			return nil, fmt.Errorf("identify resource %d: %w", index, err)
		}
		if seen.Has(identity) {
			return nil, fmt.Errorf("%w: %s/%s %s", ErrDuplicateIdentity, identity.Namespace, identity.Name, identity.GVK)
		}
		seen.Insert(identity)
	}

	objects = a.options.Sort(objects)
	values.Resources.Set(objects)

	return objects, nil
}
