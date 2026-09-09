package reconciler

import (
	"errors"
	"fmt"
	"slices"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
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

func (r *Reconciler) request(instance api.PlatformObject) *pipeline.Request {
	return &pipeline.Request{
		Client:    r.client,
		Instance:  instance,
		Resources: resources.New(nil),
		Extensions: pipeline.Extension{
			pipeline.ExtensionControllerName: r.options.ControllerName,
			pipeline.ExtensionFieldOwner:     r.options.FieldOwner,
		},
	}
}

func aggregateConditions(
	accessor api.ConditionsAccessor,
	conditionTypes []api.ConditionType,
) {
	dependentTypes := []string{string(api.ConditionTypeProvisioningSucceeded)}
	for _, conditionType := range conditionTypes {
		if conditionType == api.ConditionTypeReady ||
			conditionType == api.ConditionTypeProvisioningSucceeded {
			continue
		}

		dependentTypes = append(dependentTypes, string(conditionType))
	}

	condition.Aggregate(
		accessor,
		string(api.ConditionTypeReady),
		slices.Compact(dependentTypes)...,
	)
}
