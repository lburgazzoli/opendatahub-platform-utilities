package platformmodule

import (
	"context"
	"fmt"
	"time"

	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func allModuleRequests(names []string) func(context.Context, client.Object) []reconcile.Request {
	return func(_ context.Context, _ client.Object) []reconcile.Request {
		requests := make([]reconcile.Request, 0, len(names))
		for _, name := range names {
			requests = append(requests, reconcile.Request{NamespacedName: types.NamespacedName{Name: name}})
		}

		return requests
	}
}

func moduleNamespace(name string) string {
	return "opendatahub-" + name + "-system"
}

func (c *Controller) requireModuleCRRemoved(ctx context.Context, module *v1alpha1.PlatformModule) error {
	definition, found := c.registry.Get(module.Spec.Module)
	if !found {
		return fmt.Errorf("%w during cleanup: %q", ErrUnknownModule, module.Spec.Module)
	}

	moduleCR := new(unstructured.Unstructured)
	moduleCR.SetGroupVersionKind(definition.GVK())
	err := c.reader.Get(ctx, types.NamespacedName{Name: definition.Config.Spec.ModuleRef.Name}, moduleCR)
	switch {
	case err == nil:
		return action.NewErrorf("waiting for %s/%s removal", definition.GVK().Kind, moduleCR.GetName()).
			Advisory().
			WithRequeueAfter(2 * time.Second)

	case apierrors.IsNotFound(err):
		return nil
	case meta.IsNoMatchError(err):
		return nil
	default:
		return fmt.Errorf("get module CR %q: %w", definition.CRDName, err)
	}
}

func (c *Controller) deleteRecordedResources(ctx context.Context, refs []v1alpha1.ResourceRef) error {
	for _, ref := range refs {
		object, err := objectFromRef(ref)
		if err != nil {
			return fmt.Errorf("decode resource %q for deletion: %w", ref.Name, err)
		}
		if retainedResource(object.GroupVersionKind()) {
			continue
		}

		err = c.writer.Delete(ctx, object, client.PropagationPolicy(metav1.DeletePropagationForeground))
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return fmt.Errorf("delete %s: %w", identityOf(object), err)
		}
	}

	return nil
}

func (c *Controller) checkRecordedResourcesRemoved(ctx context.Context, refs []v1alpha1.ResourceRef) error {
	for _, ref := range refs {
		object, err := objectFromRef(ref)
		if err != nil {
			return fmt.Errorf("decode resource %q to check deletion: %w", ref.Name, err)
		}
		if retainedResource(object.GroupVersionKind()) {
			continue
		}

		err = c.reader.Get(ctx, client.ObjectKeyFromObject(object), object)
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return fmt.Errorf("check deletion of %s: %w", identityOf(object), err)
		default:
			return action.NewErrorf("waiting for %s removal", identityOf(object)).
				Advisory().WithRequeueAfter(2 * time.Second)
		}
	}

	return nil
}

func resourceRef(object *unstructured.Unstructured) v1alpha1.ResourceRef {
	return v1alpha1.ResourceRef{
		APIVersion: object.GetAPIVersion(),
		Kind:       object.GetKind(),
		Namespace:  object.GetNamespace(),
		Name:       object.GetName(),
	}
}

func objectFromRef(ref v1alpha1.ResourceRef) (*unstructured.Unstructured, error) {
	groupVersion, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return nil, fmt.Errorf("%w: apiVersion %q: %w", ErrInvalidResourceRef, ref.APIVersion, err)
	}
	if ref.Kind == "" || ref.Name == "" {
		return nil, fmt.Errorf("%w: %+v", ErrInvalidResourceRef, ref)
	}

	object := new(unstructured.Unstructured)
	object.SetGroupVersionKind(groupVersion.WithKind(ref.Kind))
	object.SetNamespace(ref.Namespace)
	object.SetName(ref.Name)

	return object, nil
}

func retainedResource(gvk schema.GroupVersionKind) bool {
	switch gvk {
	case kubegvk.Namespace:
		return true
	case kubegvk.CustomResourceDefinition:
		return true
	default:
		return false
	}
}

func identityOf(object *unstructured.Unstructured) resources.Identity {
	return resources.Identity{
		GVK:       object.GroupVersionKind(),
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}
}
