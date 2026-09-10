package requirements

import (
	"context"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

func (a *RequireAPIsAction) Name() string {
	if a == nil || a.action == nil {
		return "require-apis"
	}

	return a.action.name
}

func (a *ForbidAPIsAction) Name() string {
	if a == nil || a.action == nil {
		return "forbid-apis"
	}

	return a.action.name
}

func (a *RequireAPIsAction) Execute(ctx context.Context, request *pipeline.Request) error {
	values, err := runOptionsFromRequest(request)
	if err != nil {
		return err
	}

	return a.Run(ctx, values...)
}

func (a *ForbidAPIsAction) Execute(ctx context.Context, request *pipeline.Request) error {
	values, err := runOptionsFromRequest(request)
	if err != nil {
		return err
	}

	return a.Run(ctx, values...)
}

var _ pipeline.Action = (*RequireAPIsAction)(nil)
var _ pipeline.Action = (*ForbidAPIsAction)(nil)
