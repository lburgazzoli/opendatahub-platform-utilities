// Package platform reconciles the selected PlatformModule resources.
package platform

import (
	"context"
	"errors"
	"fmt"
	"strings"

	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var (
	ErrInvalidPlatformConfig = errors.New("invalid Platform controller configuration")
	ErrUnknownModule         = errors.New("unknown module")
)

// PlatformController renders the selected PlatformModule resources.
type PlatformController struct {
	registry *modules.Registry
}

func Setup(manager manager.Manager, registry *modules.Registry) error {
	if registry == nil {
		return fmt.Errorf("%w: registry is required", ErrInvalidPlatformConfig)
	}

	controller := &PlatformController{registry: registry}

	return reconciler.For(manager, v1alpha1.NewPlatform(),
		reconciler.WithConditionTypes(v1alpha1.ConditionModulesReady),
	).
		Owns(v1alpha1.NewPlatformModule(),
			reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		WithActionFunc(controller.render).
		WithAction(deploy.New()).
		WithActionFunc(controller.pruneModules).
		WithActionFunc(controller.updateStatus).
		Build()
}

func (c *PlatformController) render(_ context.Context, request *pipeline.Request) error {
	platform, err := reconciler.Instance[*v1alpha1.Platform](request)
	if err != nil {
		return fmt.Errorf("get Platform for rendering: %w", err)
	}

	objects := make(resources.List, 0, len(platform.Spec.Modules))
	for _, name := range platform.Spec.Modules {
		if _, found := c.registry.Get(name); !found {
			return fmt.Errorf("%w: %q", ErrUnknownModule, name)
		}

		module := v1alpha1.NewPlatformModule()
		module.Name = name
		module.Spec.Module = name

		object, err := resources.ToUnstructured(module)
		if err != nil {
			return fmt.Errorf("convert module %q: %w", name, err)
		}

		objects = append(objects, *object)
	}

	request.Resources.Set(objects)

	return nil
}

func (c *PlatformController) pruneModules(ctx context.Context, request *pipeline.Request) error {
	platform, err := reconciler.Instance[*v1alpha1.Platform](request)
	if err != nil {
		return fmt.Errorf("get Platform for pruning: %w", err)
	}

	moduleList := new(v1alpha1.PlatformModuleList)
	err = request.Client.List(ctx, moduleList)
	if err != nil {
		return fmt.Errorf("list PlatformModules: %w", err)
	}

	selected := sets.New(platform.Spec.Modules...)
	for index := range moduleList.Items {
		module := &moduleList.Items[index]
		if selected.Has(module.Name) || !module.DeletionTimestamp.IsZero() {
			continue
		}

		err = request.Client.Delete(ctx, module, client.PropagationPolicy(metav1.DeletePropagationForeground))
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return fmt.Errorf("delete PlatformModule %q: %w", module.Name, err)
		}
	}

	return nil
}

func (c *PlatformController) updateStatus(ctx context.Context, request *pipeline.Request) error {
	platform, err := reconciler.Instance[*v1alpha1.Platform](request)
	if err != nil {
		return fmt.Errorf("get Platform for status: %w", err)
	}

	moduleList := new(v1alpha1.PlatformModuleList)
	err = request.Client.List(ctx, moduleList)
	if err != nil {
		return fmt.Errorf("list PlatformModules for status: %w", err)
	}

	missing := sets.New(platform.Spec.Modules...)
	notReady := sets.New[string]()
	for index := range moduleList.Items {
		module := &moduleList.Items[index]
		if !missing.Has(module.Name) {
			continue
		}

		missing.Delete(module.Name)
		if !module.DeletionTimestamp.IsZero() ||
			module.Status.ObservedGeneration != module.Generation ||
			!condition.IsTrue(module.GetStatus(), string(platformapi.ConditionTypeReady)) {
			notReady.Insert(module.Name)
		}
	}

	notReady.Insert(missing.UnsortedList()...)
	if len(notReady) > 0 {
		condition.MarkFalse(platform.GetStatus(), string(v1alpha1.ConditionModulesReady),
			condition.WithReason("ModulesNotReady"),
			condition.WithMessagef("modules not ready: %s", strings.Join(sets.List(notReady), ", ")),
			condition.WithObservedGeneration(platform.Generation),
		)

		return nil
	}

	condition.MarkTrue(platform.GetStatus(), string(v1alpha1.ConditionModulesReady),
		condition.WithReason("AllModulesReady"),
		condition.WithObservedGeneration(platform.Generation),
	)

	return nil
}

var _ platformapi.PlatformObject = (*v1alpha1.Platform)(nil)
