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
	kubernetesClient client.Client,
	object client.Object,
	options ...client.ApplyOption,
) error {
	gvk, err := EnsureGroupVersionKind(kubernetesClient.Scheme(), object)
	if err != nil {
		return fmt.Errorf("ensure GVK: %w", err)
	}

	unstructuredObject, err := ToUnstructured(object)
	if err != nil {
		return err
	}

	applyObject := unstructuredObject.DeepCopy()
	unstructured.RemoveNestedField(applyObject.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(applyObject.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(applyObject.Object, "status")

	options = append(options, client.ForceOwnership)

	err = kubernetesClient.Apply(ctx, client.ApplyConfigurationFromUnstructured(applyObject), options...)
	if err != nil {
		return fmt.Errorf("apply %s: %w", gvk, err)
	}

	err = kubernetesClient.Scheme().Convert(applyObject, object, ctx)
	if err != nil {
		return fmt.Errorf("copy applied %s: %w", gvk, err)
	}

	return nil
}

// ApplyStatus server-side applies an object's status subresource.
func ApplyStatus(
	ctx context.Context,
	kubernetesClient client.Client,
	object client.Object,
	options ...client.SubResourceApplyOption,
) error {
	gvk, err := EnsureGroupVersionKind(kubernetesClient.Scheme(), object)
	if err != nil {
		return fmt.Errorf("ensure GVK: %w", err)
	}

	unstructuredObject, err := ToUnstructured(object)
	if err != nil {
		return err
	}

	applyObject := unstructuredObject.DeepCopy()
	unstructured.RemoveNestedField(applyObject.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(applyObject.Object, "metadata", "resourceVersion")

	options = append(options, client.ForceOwnership)

	err = kubernetesClient.Status().Apply(ctx, client.ApplyConfigurationFromUnstructured(applyObject), options...)
	if err != nil {
		return fmt.Errorf("apply status %s: %w", gvk, err)
	}

	err = kubernetesClient.Scheme().Convert(applyObject, object, ctx)
	if err != nil {
		return fmt.Errorf("copy applied status %s: %w", gvk, err)
	}

	return nil
}
