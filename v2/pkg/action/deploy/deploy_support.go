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

func (a *Action) own(
	values RunOptions,
	desired *unstructured.Unstructured,
) error {
	switch {
	case desired.GroupVersionKind() == gvk.CustomResourceDefinition:
		return nil
	case slices.Contains(a.options.ExcludeFromOwnership, desired.GroupVersionKind()):
		return nil
	default:
		desired.SetOwnerReferences(nil)
		err := ownership.SetControllerReference(values.Owner, desired, values.Client.Scheme())
		if err != nil {
			return fmt.Errorf("set controller owner: %w", err)
		}
	}

	return nil
}

func (a *Action) customize(
	ctx context.Context,
	kubernetesClient client.Client,
	desired *unstructured.Unstructured,
	current *unstructured.Unstructured,
) error {
	err := a.options.ApplyCustomizers[desired.GroupVersionKind()].Apply(
		ctx,
		kubernetesClient,
		desired,
		current,
	)
	if err != nil {
		return fmt.Errorf("apply customizer %s: %w", desired.GroupVersionKind(), err)
	}

	return nil
}

func (a *Action) apply(
	ctx context.Context,
	kubernetesClient client.Client,
	values RunOptions,
	desired *unstructured.Unstructured,
) error {
	fieldOwner := values.FieldOwner
	if fieldOwner == "" {
		fieldOwner = a.options.FieldOwner(values.Owner)
	}
	if fieldOwner == "" {
		return ErrFieldOwner
	}

	if err := resources.Apply(
		ctx,
		kubernetesClient,
		desired,
		client.FieldOwner(fieldOwner),
		client.ForceOwnership,
	); err != nil {
		return err
	}

	return nil
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
