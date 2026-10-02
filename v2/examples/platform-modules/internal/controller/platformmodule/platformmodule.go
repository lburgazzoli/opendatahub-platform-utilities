// Package platformmodule installs and removes each module controller.
package platformmodule

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	ErrInvalidConfig         = errors.New("invalid PlatformModule controller configuration")
	ErrInvalidPlatformModule = errors.New("invalid PlatformModule")
	ErrInvalidResourceRef    = errors.New("invalid PlatformModule resource reference")
	ErrUnknownModule         = errors.New("unknown module")
)

// Controller deploys a module's chart and tracks its resources.
type Controller struct {
	registry  *modules.Registry
	renderers map[string]*manifestengine.Engine
	reader    client.Reader
	image     string
}

func Setup(manager manager.Manager, registry *modules.Registry, image string) error {
	if registry == nil || image == "" {
		return fmt.Errorf("%w: registry and image are required", ErrInvalidConfig)
	}

	controller := &Controller{
		registry:  registry,
		renderers: make(map[string]*manifestengine.Engine),
		reader:    manager.GetAPIReader(),
		image:     image,
	}
	for _, name := range registry.Names() {
		definition, _ := registry.Get(name)
		namespace := moduleNamespace(name)
		if errs := validation.IsDNS1123Label(namespace); len(errs) > 0 {
			return fmt.Errorf("%w: namespace %q: %v", ErrInvalidConfig, namespace, errs)
		}

		renderer, err := helm.NewEngine(
			helm.Source{
				Chart:               definition.Chart,
				ReleaseName:         cmp.Or(definition.Config.Spec.Chart.Name, name),
				ReleaseNamespace:    namespace,
				ReleaseVersion:      definition.Config.Spec.Chart.Version,
				ProcessDependencies: true,
			},
			helm.WithCache(),
		)
		if err != nil {
			return fmt.Errorf("create module %q renderer: %w", name, err)
		}

		controller.renderers[name] = renderer
	}

	return reconciler.For(manager, v1alpha1.NewPlatformModule(), reconciler.WithCleanupTimeout(0)).
		Owns(new(appsv1.Deployment)).
		WithActionFunc(controller.render).
		WithAction(deploy.New()).
		WithActionFunc(controller.pruneOrphans).
		WithActionFunc(controller.recordResources).
		WithCleanupActionFunc(controller.cleanup).
		Build()
}

func (c *Controller) render(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return err
	}

	definition, found := c.registry.Get(module.Spec.Module)
	if !found {
		return fmt.Errorf("%w: module %q is not configured", ErrInvalidPlatformModule, module.Spec.Module)
	}
	if module.Name != module.Spec.Module {
		return fmt.Errorf("%w: %q must be named %q", ErrInvalidPlatformModule, module.Name, module.Spec.Module)
	}

	moduleValues, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&definition.Config.Spec)
	if err != nil {
		return fmt.Errorf("convert module %q spec: %w", module.Spec.Module, err)
	}
	moduleValues["enabled"] = true
	moduleValues["namespace"] = moduleNamespace(module.Spec.Module)
	moduleValues["image"] = c.image

	values := manifesttypes.Values{
		"module":      moduleValues,
		"projections": map[string]any{"enabled": false},
	}

	rendered, err := c.renderers[module.Spec.Module].Render(ctx, manifestrender.WithValues(values))
	if err != nil {
		return fmt.Errorf("render module %q chart: %w", module.Spec.Module, err)
	}

	request.Resources.Set(rendered)

	return nil
}

func (c *Controller) pruneOrphans(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return err
	}

	current := make(map[v1alpha1.ResourceRef]struct{}, request.Resources.Len())
	for _, object := range request.Resources.All() {
		current[resourceRef(object)] = struct{}{}
	}

	for _, ref := range module.Status.Resources {
		if _, found := current[ref]; found {
			continue
		}

		object, err := objectFromRef(ref)
		if err != nil {
			return err
		}
		if retainedResource(object.GroupVersionKind()) {
			continue
		}

		err = request.Client.Delete(ctx, object, client.PropagationPolicy(metav1.DeletePropagationForeground))
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return fmt.Errorf("prune %s: %w", identityOf(object), err)
		}
	}

	return nil
}

func (c *Controller) recordResources(_ context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return err
	}

	refs := make([]v1alpha1.ResourceRef, 0, request.Resources.Len())
	current := make(map[v1alpha1.ResourceRef]struct{}, request.Resources.Len())
	for _, object := range request.Resources.All() {
		ref := resourceRef(object)
		refs = append(refs, ref)
		current[ref] = struct{}{}
	}

	for _, ref := range module.Status.Resources {
		if _, found := current[ref]; found {
			continue
		}

		object, err := objectFromRef(ref)
		if err != nil {
			return err
		}
		if retainedResource(object.GroupVersionKind()) {
			refs = append(refs, ref)
		}
	}

	module.Status.Resources = refs

	return nil
}

func (c *Controller) cleanup(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return err
	}

	err = c.requireModuleCRRemoved(ctx, module)
	if err != nil {
		return err
	}

	err = deleteRecordedResources(ctx, request.Client, module.Status.Resources)
	if err != nil {
		return err
	}

	return c.checkRecordedResourcesRemoved(ctx, module.Status.Resources)
}

var _ platformapi.PlatformObject = (*v1alpha1.PlatformModule)(nil)
