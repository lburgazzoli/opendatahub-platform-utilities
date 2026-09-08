package reconciler

import (
	"context"
	"fmt"
	"slices"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *Reconciler) applyStatus(
	ctx context.Context,
	instance api.PlatformObject,
	outcome action.ActionError,
) error {
	status := instance.GetStatus()
	if status == nil {
		return fmt.Errorf("%w: %T", ErrStatusRequired, instance)
	}

	generation := instance.GetGeneration()
	status.ObservedGeneration = generation

	if accessor, ok := instance.(api.ConditionsAccessor); ok {
		markProvisioning(accessor, outcome, generation)
		aggregateConditions(accessor)
	}

	if accessor, ok := instance.(api.PhaseStatusAccessor); ok {
		phase := api.PhaseReady
		switch {
		case outcome.Err() == nil:
			phase = api.PhaseReady
		case outcome.Type() == action.ErrorTypeAdvisory:
			phase = api.PhaseReady
		default:
			phase = api.PhaseNotReady
		}

		accessor.SetPhaseStatus(api.PhaseStatus{Phase: phase})
	}

	if accessor, ok := instance.(api.PlatformProfileAccessor); ok && r.options.PlatformProfile != nil {
		accessor.SetPlatformProfile(*r.options.PlatformProfile.DeepCopy())
	}

	return resources.ApplyStatus(
		ctx,
		r.client,
		instance,
		client.FieldOwner(r.options.FieldOwner),
		client.ForceOwnership,
	)
}

func aggregateConditions(accessor api.ConditionsAccessor) {
	dependentTypes := make([]string, 0, len(accessor.GetConditions()))
	for _, current := range accessor.GetConditions() {
		if current.Type == string(api.ConditionTypeReady) {
			continue
		}

		dependentTypes = append(dependentTypes, current.Type)
	}

	condition.Aggregate(
		accessor,
		string(api.ConditionTypeReady),
		slices.Compact(dependentTypes)...,
	)
}

func markProvisioning(
	accessor api.ConditionsAccessor,
	outcome action.ActionError,
	generation int64,
) {
	markOptions := []condition.MarkOption{condition.WithObservedGeneration(generation)}

	switch {
	case outcome.Err() == nil:
		condition.MarkTrue(accessor, string(api.ConditionTypeProvisioningSucceeded), markOptions...)
	case outcome.Type() == action.ErrorTypeAdvisory:
		markOptions = append(markOptions,
			condition.WithReason(advisoryReason),
			condition.WithMessage("%s", outcome.Error()),
		)
		condition.MarkTrue(accessor, string(api.ConditionTypeProvisioningSucceeded), markOptions...)
	default:
		markOptions = append(markOptions, condition.WithError(outcome))
		condition.MarkFalse(accessor, string(api.ConditionTypeProvisioningSucceeded), markOptions...)
	}
}
