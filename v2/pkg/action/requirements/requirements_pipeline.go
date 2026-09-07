package requirements

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

var (
	ErrRequestRequired  = errors.New("requirements pipeline request is required")
	ErrInstanceRequired = errors.New("requirements instance is required")
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

func runOptionsFromRequest(request *pipeline.Request) ([]RunOption, error) {
	if request == nil {
		return nil, ErrRequestRequired
	}
	if request.Instance == nil {
		return nil, ErrInstanceRequired
	}
	if request.Client == nil {
		return nil, ErrClientRequired
	}

	conditions, ok := request.Instance.(api.ConditionsAccessor)
	if !ok {
		return nil, fmt.Errorf("%w: %T", ErrConditionsRequired, request.Instance)
	}

	return []RunOption{
		WithClient(request.Client),
		WithConditions(conditions),
		WithObservedGeneration(request.Instance.GetGeneration()),
	}, nil
}

var _ pipeline.Action = (*RequireAPIsAction)(nil)
var _ pipeline.Action = (*ForbidAPIsAction)(nil)
