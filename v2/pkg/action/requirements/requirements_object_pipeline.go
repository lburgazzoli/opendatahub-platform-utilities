package requirements

import (
	"context"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

func (a *RequireObjectsAction) Name() string {
	if a == nil || a.action == nil {
		return "require-objects"
	}

	return a.action.name
}

func (a *ForbidObjectsAction) Name() string {
	if a == nil || a.action == nil {
		return "forbid-objects"
	}

	return a.action.name
}

func (a *RequireObjectsAction) Execute(ctx context.Context, request *pipeline.Request) error {
	values, err := runOptionsFromRequest(request)
	if err != nil {
		return err
	}

	return a.Run(ctx, values...)
}

func (a *ForbidObjectsAction) Execute(ctx context.Context, request *pipeline.Request) error {
	values, err := runOptionsFromRequest(request)
	if err != nil {
		return err
	}

	return a.Run(ctx, values...)
}

var _ pipeline.Action = (*RequireObjectsAction)(nil)
var _ pipeline.Action = (*ForbidObjectsAction)(nil)
