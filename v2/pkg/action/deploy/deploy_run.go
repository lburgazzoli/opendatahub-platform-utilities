package deploy

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

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
	if a.options.Sort == nil {
		return ErrSort
	}
	return nil
}

// Run decorates and applies the desired resources.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Result, error) {
	err := a.Validate()
	if err != nil {
		return Result{}, err
	}

	ro := RunOptions{}
	ro.Merge(values...)

	if err := ro.Validate(); err != nil {
		return Result{}, err
	}
	if a.cache != nil {
		a.cache.Sync()
	}

	ro.Resources.Sort(a.options.Sort)

	result := Result{}
	var runErrors []error
	for _, object := range ro.Resources.All() {
		resources.SetLabels(object, a.options.Labels)
		resources.SetAnnotations(object, a.options.Annotations)
		resources.SetLabels(object, ro.Labels)
		resources.SetAnnotations(object, ro.Annotations)

		var applied bool
		err := a.options.MetadataPolicy.Apply(object, ro.Owner)
		if err != nil {
			err = fmt.Errorf("decorate %s/%s %s: %w", object.GetNamespace(), object.GetName(), object.GroupVersionKind(), err)
		} else {
			applied, err = a.deploy(ctx, ro, object)
		}

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

// deploy processes a decorated object through lookup, ownership,
// customization, cache evaluation, and server-side apply.
func (a *Action) deploy(
	ctx context.Context,
	values RunOptions,
	object *unstructured.Unstructured,
) (bool, error) {
	// Resolve the desired and current objects before ownership and customization.
	desired := object.DeepCopy()

	current, err := a.lookupCurrent(ctx, values.Client, desired)
	if err != nil {
		return false, err
	}

	skip, err := a.shouldSkip(current, desired)
	if err != nil {
		return false, err
	}
	if skip {
		return false, nil
	}

	// Add the controller owner when the resource policy permits ownership.
	if err := a.own(values, desired); err != nil {
		return false, err
	}

	// Apply the resource-specific customization using the current object.
	if err := a.customize(ctx, values.Client, desired, current); err != nil {
		return false, err
	}

	// Skip SSA when the desired state is already represented by the cache.
	if a.cache != nil {
		skip, err = a.cache.Has(current, desired)
		if err != nil {
			return false, err
		}
		if skip {
			return false, nil
		}
	}

	// Keep the pre-apply desired state as the cache fingerprint.
	var cacheDesired *unstructured.Unstructured
	if a.cache != nil {
		cacheDesired = desired.DeepCopy()
	}

	// Apply the fully prepared object with the resolved field owner.
	if err := a.apply(ctx, values.Client, values, desired); err != nil {
		return false, err
	}

	// Record the post-apply desired object for future cache checks.
	if a.cache != nil {
		err = a.cache.Add(desired, cacheDesired)
		if err != nil {
			return false, fmt.Errorf("cache deployed resource: %w", err)
		}
	}

	return true, nil
}
