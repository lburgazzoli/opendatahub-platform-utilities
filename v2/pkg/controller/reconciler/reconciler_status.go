package reconciler

import (
	"context"
	"fmt"

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
		aggregateConditions(accessor, r.options.ConditionTypes)
	}

	if accessor, ok := instance.(api.PhaseStatusAccessor); ok {
		phase := api.PhaseNotReady
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

func markProvisioning(
	accessor api.ConditionsAccessor,
	outcome action.ActionError,
	generation int64,
) {
	switch {
	case outcome.Err() == nil:
		condition.MarkTrue(
			accessor,
			string(api.ConditionTypeProvisioningSucceeded),
			condition.WithObservedGeneration(generation),
		)
	case outcome.Type() == action.ErrorTypeAdvisory:
		condition.MarkTrue(
			accessor,
			string(api.ConditionTypeProvisioningSucceeded),
			condition.WithObservedGeneration(generation),
			condition.WithReason(advisoryReason),
			condition.WithMessage(outcome.Error()),
		)
	default:
		condition.MarkFalse(
			accessor,
			string(api.ConditionTypeProvisioningSucceeded),
			condition.WithObservedGeneration(generation),
			condition.WithError(outcome),
		)
	}
}
