package cluster_test

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var errDiscoveryClient = errors.New("discovery client failure") //nolint:gochecknoglobals // Shared test error.

type errorReader struct {
	client.Reader
	err error
}

func (r errorReader) Get(
	context.Context,
	client.ObjectKey,
	client.Object,
	...client.GetOption,
) error {
	return r.err
}

func object(
	gvk schema.GroupVersionKind,
	namespace string,
	name string,
	fields map[string]any,
) *unstructured.Unstructured {
	value := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}}

	for key, field := range fields {
		value.Object[key] = field
	}

	return value
}
