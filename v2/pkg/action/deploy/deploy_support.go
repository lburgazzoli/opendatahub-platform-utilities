package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
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
	if current != nil && resources.HasAnnotation(current, a.options.ManagedAnnotation, "false") {
		return false, nil
	}

	skip, err := a.shouldSkip(current, desired)
	if err != nil {
		return false, err
	}
	if skip {
		return false, nil
	}

	if desired.GroupVersionKind() != gvk.CustomResourceDefinition &&
		!slices.Contains(a.options.ExcludeFromOwnership, desired.GroupVersionKind()) {
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
	deployed, err := a.write(ctx, values.Client, desired, current, fieldOwner)
	if err != nil {
		return false, err
	}
	if a.cache != nil {
		err = a.cache.Add(deployed, desired)
		if err != nil {
			return false, fmt.Errorf("cache deployed resource: %w", err)
		}
	}

	return true, nil
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
	if current != nil && !current.GetDeletionTimestamp().IsZero() {
		if a.cache == nil {
			return true, nil
		}
		err := a.cache.Delete(current, desired)
		if err != nil {
			return false, err
		}
		return true, nil
	}
	if a.cache == nil {
		return false, nil
	}
	return a.cache.Has(current, desired)
}

func (a *Action) write(
	ctx context.Context,
	kubernetesClient client.Client,
	desired *unstructured.Unstructured,
	current *unstructured.Unstructured,
	fieldOwner string,
) (*unstructured.Unstructured, error) {
	switch a.options.Mode {
	case ModePatch:
		return a.writePatch(ctx, kubernetesClient, desired, current, fieldOwner)
	case ModeSSA:
		return a.writeSSA(ctx, kubernetesClient, desired, current, fieldOwner)
	default:
		return nil, fmt.Errorf("%w: %d", ErrUnsupportedMode, a.options.Mode)
	}
}

//nolint:cyclop // SSA combines merge, customization, aggregation, and apply semantics.
func (a *Action) writeSSA(
	ctx context.Context,
	kubernetesClient client.Client,
	desired *unstructured.Unstructured,
	current *unstructured.Unstructured,
	fieldOwner string,
) (*unstructured.Unstructured, error) {
	var err error
	if current != nil && !resources.HasAnnotation(current, a.options.ManagedAnnotation, "true") {
		if merge := a.options.MergeStrategies[desired.GroupVersionKind()]; merge != nil {
			err = merge(current, desired)
			if err != nil {
				return nil, fmt.Errorf("merge %s: %w", desired.GroupVersionKind(), err)
			}
		}
	}

	customizer := a.options.ApplyCustomizers[desired.GroupVersionKind()]
	if customizer == nil {
		customizer = applyCoreCustomizer
	}
	err = customizer(ctx, kubernetesClient, a, desired, current)
	if err != nil {
		return nil, fmt.Errorf("apply customizer %s: %w", desired.GroupVersionKind(), err)
	}
	err = resources.Apply(
		ctx,
		kubernetesClient,
		desired,
		client.ForceOwnership,
		client.FieldOwner(fieldOwner),
	)
	if err != nil {
		return nil, err
	}
	return desired, nil
}

func (a *Action) writePatch(
	ctx context.Context,
	kubernetesClient client.Client,
	desired *unstructured.Unstructured,
	current *unstructured.Unstructured,
	fieldOwner string,
) (*unstructured.Unstructured, error) {
	var err error
	if customizer := a.options.PatchCustomizers[desired.GroupVersionKind()]; customizer != nil {
		err = customizer(ctx, kubernetesClient, a, desired, current)
		if err != nil {
			return nil, fmt.Errorf("patch customizer %s: %w", desired.GroupVersionKind(), err)
		}
	}
	if current == nil {
		err = kubernetesClient.Create(ctx, desired)
		if err != nil {
			return nil, fmt.Errorf("create %s/%s: %w", desired.GetNamespace(), desired.GetName(), err)
		}
		return desired, nil
	}
	var data []byte
	data, err = json.Marshal(desired)
	if err != nil {
		return nil, fmt.Errorf("marshal patch %s/%s: %w", desired.GetNamespace(), desired.GetName(), err)
	}
	err = kubernetesClient.Patch(
		ctx,
		current,
		client.RawPatch(types.ApplyPatchType, data),
		client.ForceOwnership,
		client.FieldOwner(fieldOwner),
	)
	if err != nil {
		return nil, fmt.Errorf("patch %s/%s: %w", desired.GetNamespace(), desired.GetName(), err)
	}
	return current, nil
}
