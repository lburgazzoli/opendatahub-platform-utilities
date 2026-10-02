package requirements

import (
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
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

	conditions := request.Instance.GetStatus()
	if conditions == nil {
		return nil, ErrConditionsRequired
	}

	return []RunOption{
		WithClient(request.Client),
		WithConditions(conditions),
		WithObservedGeneration(request.Instance.GetGeneration()),
	}, nil
}
