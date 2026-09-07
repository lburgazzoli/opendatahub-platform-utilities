package resources

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Apply server-side applies an object after removing server-owned fields.
func Apply(
	ctx context.Context,
	cli client.Client,
	object client.Object,
	options ...client.ApplyOption,
) error {
	gvk, err := EnsureGroupVersionKind(cli.Scheme(), object)
	if err != nil {
		return fmt.Errorf("ensure GVK: %w", err)
	}

	u, err := ToUnstructured(object)
	if err != nil {
		return err
	}

	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(u.Object, "status")

	if err := cli.Apply(
		ctx,
		client.ApplyConfigurationFromUnstructured(u),
		options...,
	); err != nil {
		return fmt.Errorf("apply %s: %w", gvk, err)
	}

	switch target := object.(type) {
	case *unstructured.Unstructured:
		target.Object = u.Object
	default:
		if err := cli.Scheme().Convert(u, object, ctx); err != nil {
			return fmt.Errorf("copy applied %s: %w", gvk, err)
		}
	}

	return nil
}

// ApplyStatus server-side applies an object's status subresource.
func ApplyStatus(
	ctx context.Context,
	cli client.Client,
	object client.Object,
	options ...client.SubResourceApplyOption,
) error {
	gvk, err := EnsureGroupVersionKind(cli.Scheme(), object)
	if err != nil {
		return fmt.Errorf("ensure GVK: %w", err)
	}

	u, err := ToUnstructured(object)
	if err != nil {
		return err
	}

	unstructured.RemoveNestedField(u.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(u.Object, "metadata", "resourceVersion")

	if err := cli.Status().Apply(
		ctx,
		client.ApplyConfigurationFromUnstructured(u),
		options...,
	); err != nil {
		return fmt.Errorf("apply status %s: %w", gvk, err)
	}

	switch target := object.(type) {
	case *unstructured.Unstructured:
		target.Object = u.Object
	default:
		if err := cli.Scheme().Convert(u, object, ctx); err != nil {
			return fmt.Errorf("copy applied status %s: %w", gvk, err)
		}
	}

	return nil
}
