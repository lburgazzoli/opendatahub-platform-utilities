package deployment

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

var (
	ErrRequestRequired  = errors.New("deployment pipeline request is required")
	ErrInstanceRequired = errors.New("deployment instance is required")
)

// Name returns the default pipeline registration name.
func (a *Action) Name() string { return "deployment-availability" }

// Execute observes Deployments and writes the condition to the instance.
func (a *Action) Execute(ctx context.Context, request *pipeline.Request) error {
	if request == nil {
		return ErrRequestRequired
	}
	if request.Instance == nil {
		return ErrInstanceRequired
	}

	conditions, ok := request.Instance.(api.ConditionsAccessor)
	if !ok {
		return fmt.Errorf("%w: %T", ErrConditionsRequired, request.Instance)
	}

	observation, err := a.Run(ctx, RunOptions{
		Client:    request.Client,
		Namespace: request.Instance.GetNamespace(),
	})
	if err != nil {
		return err
	}

	condition.Set(conditions, observation.Condition)

	return nil
}

var _ pipeline.Action = (*Action)(nil)
