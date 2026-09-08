package deploy

import (
	"context"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

// Execute maps the pipeline request to the programmatic deploy contract.
func (a *Action) Execute(ctx context.Context, request *pipeline.Request) error {
	_, err := a.Run(ctx, runOptionsFromRequest(request))
	return err
}

func runOptionsFromRequest(request *pipeline.Request) RunOptions {
	options := RunOptions{
		Client:    request.Client,
		Owner:     request.Instance,
		Resources: request.Resources,
	}
	if fieldOwner, ok := request.Extensions[pipeline.ExtensionFieldOwner].(string); ok {
		options.FieldOwner = fieldOwner
	}

	return options
}
