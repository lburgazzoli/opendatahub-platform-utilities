package deletion

import (
	"context"
	"errors"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

var ErrRequestRequired = errors.New("delete pipeline request is required")

// Name returns the default pipeline registration name.
func (a *Action) Name() string { return "delete" }

// Execute maps the pipeline request to the programmatic delete contract.
func (a *Action) Execute(ctx context.Context, request *pipeline.Request) error {
	if request == nil {
		return ErrRequestRequired
	}
	if request.Instance == nil {
		return ErrInstanceRequired
	}

	_, err := a.Run(ctx, RunOptions{
		Client:    request.Client,
		Namespace: request.Instance.GetNamespace(),
	})

	return err
}

var _ pipeline.Action = (*Action)(nil)
