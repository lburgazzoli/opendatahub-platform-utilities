package module

import (
	"context"
	"errors"
	"fmt"

	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const SimulationAnnotation = "example.platform.odh.io/simulated-failure"

const (
	ConditionModuleConfigured = "ModuleConfigured"
	ConditionSimulationActive = "SimulationActive"
)

var ErrSimulatedFailure = errors.New("simulated module failure")

// ModuleController reports health for a typed module CR.
type ModuleController struct{}

func Setup(manager manager.Manager, obj platformapi.PlatformObject) error {
	controller := &ModuleController{}

	err := reconciler.For(manager, obj).
		WithActionFunc(controller.updateStatus, pipeline.WithName("update-status")).
		Build()

	if err != nil {
		return err
	}

	return nil
}

func (*ModuleController) updateStatus(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[platformapi.PlatformObject](request)
	if err != nil {
		return err
	}

	condition.MarkTrue(module.GetStatus(), ConditionModuleConfigured,
		condition.WithReason("Registered"),
		condition.WithMessage(module.GetObjectKind().GroupVersionKind().String()),
		condition.WithObservedGeneration(module.GetGeneration()),
	)

	switch message := resources.GetAnnotation(module, SimulationAnnotation); {
	case message != "":
		condition.MarkTrue(module.GetStatus(), ConditionSimulationActive,
			condition.WithReason("Requested"),
			condition.WithMessage(message),
			condition.WithObservedGeneration(module.GetGeneration()),
		)

		return fmt.Errorf("%w: %s", ErrSimulatedFailure, message)
	default:
		condition.MarkFalse(module.GetStatus(), ConditionSimulationActive,
			condition.WithReason("Disabled"),
			condition.WithObservedGeneration(module.GetGeneration()),
		)
	}

	return ctx.Err()
}
