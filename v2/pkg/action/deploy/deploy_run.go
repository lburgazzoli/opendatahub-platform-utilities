package deploy

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	if a.options.Sort == nil {
		return ErrSort
	}
	return nil
}

// Run normalizes, decorates, and applies the desired resources.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Result, error) {
	err := a.Validate()
	if err != nil {
		return Result{}, err
	}
	var runOptions RunOptions
	runOptions.Merge(values...)
	if err := runOptions.Validate(); err != nil {
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
	for index := range objects {
		object := &objects[index]
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

// deployOne processes one desired object through lookup, policy, customization,
// cache evaluation, and server-side apply.
func (a *Action) deployOne(
	ctx context.Context,
	values RunOptions,
	object *unstructured.Unstructured,
) (bool, error) {
	// Resolve the desired and current objects before applying any policy.
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

func (a *Action) prepare(values RunOptions) (resources.List, error) {
	seen := sets.New[resources.Identity]()

	objects := make(resources.List, 0, values.Resources.Len())
	for _, res := range values.Resources.All() {
		object := res.DeepCopy()
		objects = append(objects, *object)

		identity, err := resources.IdentityOf(object, values.Client.Scheme())
		switch {
		case err != nil:
			return nil, fmt.Errorf("identify resource: %w", err)
		case seen.Has(identity):
			return nil, fmt.Errorf("%w: %s/%s %s", ErrDuplicateIdentity, identity.Namespace, identity.Name, identity.GVK)
		default:
			seen.Insert(identity)
		}

		resources.SetLabels(object, a.options.Labels)
		resources.SetAnnotations(object, a.options.Annotations)
		resources.SetLabels(object, values.Labels)
		resources.SetAnnotations(object, values.Annotations)

		err = a.options.MetadataPolicy.Apply(object, values.Owner)
		if err != nil {
			return nil, fmt.Errorf("decorate %s/%s %s: %w", object.GetNamespace(), object.GetName(), identity.GVK, err)
		}
	}

	a.options.Sort(objects)
	values.Resources.Set(objects)

	return objects, nil
}
