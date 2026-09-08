// Package client provides the cache-coherent controller-runtime client used by
// the v2 controller examples and manager wrapper.
package client

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Client converts typed cache reads to unstructured reads while delegating
// writes to the wrapped controller-runtime client.
type Client struct {
	inner client.Client
}

func (c *Client) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	opts ...client.GetOption,
) error {
	log := logf.FromContext(ctx)

	gvk, err := apiutil.GVKForObject(object, c.Scheme())
	if err != nil {
		return fmt.Errorf("get GVK: %w", err)
	}

	_, isUnstructured := object.(*unstructured.Unstructured)
	_, isPartialMeta := object.(*metav1.PartialObjectMetadata)
	if isUnstructured || isPartialMeta {
		log.V(1).Info("client Get uses matching cache type", "gvk", gvk, "key", key)
		return c.inner.Get(ctx, key, object, opts...)
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	err = c.inner.Get(ctx, key, u, opts...)
	if err != nil {
		return err
	}

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, object)
	if err != nil {
		return fmt.Errorf("convert unstructured object: %w", err)
	}

	object.GetObjectKind().SetGroupVersionKind(gvk)

	return nil
}

func (c *Client) List(
	ctx context.Context,
	list client.ObjectList,
	opts ...client.ListOption,
) error {
	log := logf.FromContext(ctx)

	gvk, err := apiutil.GVKForObject(list, c.Scheme())
	if err != nil {
		return fmt.Errorf("get list GVK: %w", err)
	}

	_, isUnstructuredList := list.(*unstructured.UnstructuredList)
	_, isPartialMetaList := list.(*metav1.PartialObjectMetadataList)
	if isUnstructuredList || isPartialMetaList || hasFieldSelector(opts) {
		log.V(1).Info("client List delegates to wrapped client", "gvk", gvk)
		return c.inner.List(ctx, list, opts...)
	}

	unstructuredList := &unstructured.UnstructuredList{}
	unstructuredList.SetGroupVersionKind(gvk)
	err = c.inner.List(ctx, unstructuredList, opts...)
	if err != nil {
		return err
	}

	itemGVK := schema.GroupVersionKind{
		Group:   gvk.Group,
		Version: gvk.Version,
		Kind:    strings.TrimSuffix(gvk.Kind, "List"),
	}
	items := make([]runtime.Object, 0, len(unstructuredList.Items))
	for index := range unstructuredList.Items {
		object, createErr := c.Scheme().New(itemGVK)
		if createErr != nil {
			return fmt.Errorf("create typed list object %d: %w", index, createErr)
		}

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(
			unstructuredList.Items[index].Object,
			object,
		)
		if err != nil {
			return fmt.Errorf("convert typed list object %d: %w", index, err)
		}

		items = append(items, object)
	}

	err = meta.SetList(list, items)
	if err != nil {
		return fmt.Errorf("set typed list items: %w", err)
	}

	list.SetResourceVersion(unstructuredList.GetResourceVersion())
	list.SetContinue(unstructuredList.GetContinue())
	list.SetRemainingItemCount(unstructuredList.GetRemainingItemCount())
	list.GetObjectKind().SetGroupVersionKind(gvk)

	return nil
}

func (c *Client) Create(ctx context.Context, object client.Object, opts ...client.CreateOption) error {
	return c.inner.Create(ctx, object, opts...)
}

func (c *Client) Delete(ctx context.Context, object client.Object, opts ...client.DeleteOption) error {
	return c.inner.Delete(ctx, object, opts...)
}

func (c *Client) Update(ctx context.Context, object client.Object, opts ...client.UpdateOption) error {
	return c.inner.Update(ctx, object, opts...)
}

func (c *Client) Patch(
	ctx context.Context,
	object client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	return c.inner.Patch(ctx, object, patch, opts...)
}

func (c *Client) Apply(
	ctx context.Context,
	configuration runtime.ApplyConfiguration,
	opts ...client.ApplyOption,
) error {
	return c.inner.Apply(ctx, configuration, opts...)
}

func (c *Client) DeleteAllOf(
	ctx context.Context,
	object client.Object,
	opts ...client.DeleteAllOfOption,
) error {
	return c.inner.DeleteAllOf(ctx, object, opts...)
}

func (c *Client) Status() client.SubResourceWriter {
	return c.inner.Status()
}

func (c *Client) SubResource(name string) client.SubResourceClient {
	return c.inner.SubResource(name)
}

func (c *Client) Scheme() *runtime.Scheme {
	return c.inner.Scheme()
}

func (c *Client) RESTMapper() meta.RESTMapper {
	return c.inner.RESTMapper()
}

func (c *Client) GroupVersionKindFor(object runtime.Object) (schema.GroupVersionKind, error) {
	return c.inner.GroupVersionKindFor(object)
}

func (c *Client) IsObjectNamespaced(object runtime.Object) (bool, error) {
	return c.inner.IsObjectNamespaced(object)
}

var _ client.Client = (*Client)(nil)
