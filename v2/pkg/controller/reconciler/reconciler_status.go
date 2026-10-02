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

	switch {
	case outcome.Err() == nil:
		condition.MarkTrue(
			status,
			string(api.ConditionTypeProvisioningSucceeded),
			condition.WithObservedGeneration(generation),
		)
	case outcome.Type() == action.ErrorTypeAdvisory:
		condition.MarkTrue(
			status,
			string(api.ConditionTypeProvisioningSucceeded),
			condition.WithObservedGeneration(generation),
			condition.WithReason(advisoryReason),
			condition.WithMessage(outcome.Error()),
		)
	default:
		condition.MarkFalse(
			status,
			string(api.ConditionTypeProvisioningSucceeded),
			condition.WithObservedGeneration(generation),
			condition.WithError(outcome),
		)
	}

	condition.Aggregate(
		status,
		api.ConditionTypeReady,
		r.options.ConditionTypes...,
	)

	if accessor, ok := instance.(api.PhaseStatusAccessor); ok {
		var phase api.Phase

		switch {
		case outcome.Err() == nil:
			phase = api.PhaseReady
		case outcome.Type() == action.ErrorTypeAdvisory:
			phase = api.PhaseReady
		default:
			phase = api.PhaseNotReady
		}

		accessor.SetPhaseStatus(api.PhaseStatus{
			Phase: phase,
		})
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
