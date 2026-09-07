// Package singleton retrieves a single cluster-scoped custom resource.
package singleton

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNoInstance        = errors.New("no singleton instance found")
	ErrMultipleInstances = errors.New("multiple singleton instances found")
	ErrUnregisteredType  = errors.New("singleton type is not registered in scheme")
)

// Get retrieves exactly one instance of target's registered GVK.
func Get[T client.Object](ctx context.Context, kubernetesClient client.Client, target T) error {
	gvks, _, err := kubernetesClient.Scheme().ObjectKinds(target)
	if err != nil || len(gvks) == 0 {
		return fmt.Errorf("resolve singleton GVK: %w", ErrUnregisteredType)
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   gvks[0].Group,
		Version: gvks[0].Version,
		Kind:    gvks[0].Kind + "List",
	})

	err = kubernetesClient.List(ctx, list)
	if err != nil {
		return fmt.Errorf("list singleton %s: %w", gvks[0], err)
	}

	switch len(list.Items) {
	case 0:
		return ErrNoInstance
	case 1:
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(list.Items[0].Object, target)
		if err != nil {
			return fmt.Errorf("convert singleton %s: %w", gvks[0], err)
		}

		return nil
	default:
		return fmt.Errorf("%s: %w", gvks[0], ErrMultipleInstances)
	}
}
