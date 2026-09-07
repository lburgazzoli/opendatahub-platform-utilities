package deploy_test

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type countingClient struct {
	client.Client
	applyCalls int
}

func (c *countingClient) Apply(
	ctx context.Context,
	object runtime.ApplyConfiguration,
	options ...client.ApplyOption,
) error {
	c.applyCalls++

	return c.Client.Apply(ctx, object, options...)
}
