// Package platform reconciles the selected PlatformModule resources.
package platform

import (
	"context"
	"errors"
	"fmt"

	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/gc"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"sigs.k8s.io/controller-runtime/pkg/manager"
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

	return reconciler.For(manager, v1alpha1.NewPlatform()).
		Owns(v1alpha1.NewPlatformModule()).
		WithActionFunc(controller.render).
		WithAction(deploy.New()).
		WithAction(gc.New(gc.StaticDiscovery(v1alpha1.PlatformModuleGVK))).
		Build()
}

func (c *PlatformController) render(_ context.Context, request *pipeline.Request) error {
	platform, err := reconciler.Instance[*v1alpha1.Platform](request)
	if err != nil {
		return err
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

var _ platformapi.PlatformObject = (*v1alpha1.Platform)(nil)
