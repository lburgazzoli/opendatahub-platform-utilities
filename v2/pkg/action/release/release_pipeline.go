package release

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

var (
	ErrRequestRequired  = errors.New("release pipeline request is required")
	ErrInstanceRequired = errors.New("release instance is required")
)

// Name returns the default pipeline registration name.
func (a *Action) Name() string { return "release" }

// Execute reads release metadata and stores it on the pipeline instance.
func (a *Action) Execute(ctx context.Context, request *pipeline.Request) error {
	if request == nil {
		return ErrRequestRequired
	}
	if request.Instance == nil {
		return ErrInstanceRequired
	}

	accessor, ok := request.Instance.(api.ReleaseStatusAccessor)
	if !ok {
		return fmt.Errorf("%w: %T", ErrReleaseStatusRequired, request.Instance)
	}

	status, err := a.Run(ctx)
	if err != nil {
		return err
	}
	accessor.SetReleaseStatus(status)

	return nil
}

var _ pipeline.Action = (*Action)(nil)
