package deploy_test

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

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
