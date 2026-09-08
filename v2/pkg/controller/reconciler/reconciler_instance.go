package reconciler

import (
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

var ErrInstanceType = errors.New("reconciler request instance has unexpected type")

// Instance returns the typed platform object carried by a pipeline request.
func Instance[T api.PlatformObject](request *pipeline.Request) (T, error) {
	var zero T
	if request == nil {
		return zero, fmt.Errorf("%w: request is nil", ErrInstanceType)
	}

	instance, ok := request.Instance.(T)
	if !ok {
		return zero, fmt.Errorf("%w: expected %T, got %T", ErrInstanceType, zero, request.Instance)
	}

	return instance, nil
}
