package deploy

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/ownership"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

//nolint:cyclop // deployment policy has ordered Kubernetes safety branches.
func (a *Action) deployOne(ctx context.Context, values RunOptions, object client.Object) (bool, error) {
	desired, err := resources.ToUnstructured(object)
	if err != nil {
		return false, err
	}
	desired.SetGroupVersionKind(object.GetObjectKind().GroupVersionKind())
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

	if a.shouldOwn(desired) {
		desired.SetOwnerReferences(nil)
		err = ownership.SetControllerReference(values.Owner, desired, values.Client.Scheme())
		if err != nil {
			return false, fmt.Errorf("set controller owner: %w", err)
		}
	}

	fieldOwner := a.options.FieldOwner(values.Owner)
	if fieldOwner == "" {
		return false, ErrFieldOwner
	}

	err = a.options.ApplyCustomizers[desired.GroupVersionKind()].Apply(
		ctx,
		values.Client,
		desired,
		current,
	)
	if err != nil {
		return false, fmt.Errorf("apply customizer %s: %w", desired.GroupVersionKind(), err)
	}

	if a.cache != nil {
		skip, err = a.cache.Has(current, desired)
		if err != nil {
			return false, err
		}
		if skip {
			return false, nil
		}
	}

	var cacheDesired *unstructured.Unstructured
	if a.cache != nil {
		cacheDesired = desired.DeepCopy()
	}

	if err := resources.Apply(
		ctx,
		values.Client,
		desired,
		client.FieldOwner(fieldOwner),
	); err != nil {
		return false, err
	}

	if a.cache != nil {
		err = a.cache.Add(desired, cacheDesired)
		if err != nil {
			return false, fmt.Errorf("cache deployed resource: %w", err)
		}
	}

	return true, nil
}

func (a *Action) shouldOwn(desired *unstructured.Unstructured) bool {
	return desired.GroupVersionKind() != gvk.CustomResourceDefinition &&
		!slices.Contains(a.options.ExcludeFromOwnership, desired.GroupVersionKind())
}

func (a *Action) lookupCurrent(
	ctx context.Context,
	kubernetesClient client.Client,
	desired *unstructured.Unstructured,
) (*unstructured.Unstructured, error) {
	current := &unstructured.Unstructured{}
	current.SetGroupVersionKind(desired.GroupVersionKind())
	err := kubernetesClient.Get(ctx, client.ObjectKeyFromObject(desired), current)
	switch {
	case apierrors.IsNotFound(err):
		return nil, nil //nolint:nilnil // absence is a successful lookup result.
	case meta.IsNoMatchError(err):
		return nil, nil //nolint:nilnil // an unavailable kind has no current object.
	case err != nil:
		return nil, fmt.Errorf("lookup %s/%s: %w", desired.GetNamespace(), desired.GetName(), err)
	default:
		return current, nil
	}
}

func (a *Action) shouldSkip(
	current *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) (bool, error) {
	switch {
	case current == nil:
		return false, nil
	case resources.HasAnnotation(current, a.options.ManagedAnnotation):
		return true, nil
	case !current.GetDeletionTimestamp().IsZero():
		if a.cache == nil {
			return true, nil
		}
		err := a.cache.Delete(current, desired)
		if err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, nil
	}
}
