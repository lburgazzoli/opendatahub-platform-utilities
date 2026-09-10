package requirements

import (
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

var (
	ErrRequestRequired  = errors.New("requirements pipeline request is required")
	ErrInstanceRequired = errors.New("requirements instance is required")
)

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
