package deploy_test

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

func resourceList(
	t *testing.T,
	scheme *runtime.Scheme,
	objects ...client.Object,
) resources.List {
	t.Helper()

	list := make(resources.List, len(objects))
	for index, object := range objects {
		u, err := resources.ToUnstructured(object)
		if err != nil {
			t.Fatal(err)
		}
		groupVersionKind := object.GetObjectKind().GroupVersionKind()
		if groupVersionKind.Empty() {
			if _, ok := object.(*unstructured.Unstructured); ok {
				list[index] = *u
				continue
			}

			groupVersionKinds, _, err := scheme.ObjectKinds(object)
			if err != nil {
				t.Fatal(err)
			}
			groupVersionKind = groupVersionKinds[0]
		}
		u.SetGroupVersionKind(groupVersionKind)
		list[index] = *u
	}

	return list
}

type countingClient struct {
	client.Client
	applyCalls int
	getCalls   int
}

func (c *countingClient) Get(
	ctx context.Context,
	key client.ObjectKey,
	object client.Object,
	options ...client.GetOption,
) error {
	c.getCalls++

	return c.Client.Get(ctx, key, object, options...)
}

func (c *countingClient) Apply(
	ctx context.Context,
	object runtime.ApplyConfiguration,
	options ...client.ApplyOption,
) error {
	c.applyCalls++

	return c.Client.Apply(ctx, object, options...)
}
