// Package platformmodule installs and removes each module controller.
package platformmodule

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	platformhandler "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	platformpredicate "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
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
	writer    client.Writer
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
		writer:    manager.GetClient(),
		image:     image,
	}

	r := reconciler.For(manager, v1alpha1.NewPlatformModule(),
		reconciler.WithCleanupTimeout(0),
		reconciler.WithExcludedOwnershipTypes(gvk.CustomResourceDefinition),
	)

	r = r.WithDynamicOwnership()

	moduleNames := registry.Names()
	for _, name := range moduleNames {
		definition, found := registry.Get(name)
		if !found {
			return fmt.Errorf("%w: %q is missing from registry", ErrUnknownModule, name)
		}

		renderer, err := newModuleRenderer(name, definition)
		if err != nil {
			return fmt.Errorf("configure module %q renderer: %w", name, err)
		}

		controller.renderers[name] = renderer

		r = r.WatchesGVK(definition.GVK(),
			reconciler.WithEventHandler(platformhandler.ToNamed(name)),
			reconciler.WithPredicates(platformpredicate.CreatedOrUpdatedOrDeletedNamed(definition.Config.Spec.ModuleRef.Name)),
			reconciler.Dynamic(reconciler.CrdExists(definition.GVK())),
		)
	}

	r = r.WatchesGVK(
		gvk.CustomResourceDefinition,
		reconciler.WithEventMapper(allModuleRequests(moduleNames)),
		reconciler.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
	)

	r = r.WithActionFunc(controller.render)
	r = r.WithAction(deploy.New())
	r = r.WithActionFunc(controller.pruneOrphans)
	r = r.WithActionFunc(controller.recordResources)
	r = r.WithCleanupActionFunc(controller.cleanup)

	err := r.Build()
	if err != nil {
		return fmt.Errorf("setup platform module controller: %w", err)
	}

	return nil
}

func (c *Controller) render(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return fmt.Errorf("get PlatformModule for rendering: %w", err)
	}

	definition, found := c.registry.Get(module.Spec.Module)
	if !found {
		return fmt.Errorf("%w: module %q is not configured", ErrInvalidPlatformModule, module.Spec.Module)
	}

	values, err := ToValues(ChartValues{
		Module: ModuleValues{
			ModuleSpec: definition.Config.Spec,
			Enabled:    true,
			Namespace:  moduleNamespace(module.Spec.Module),
			Image:      c.image,
		},
		Projections: ProjectionValues{
			Enabled: false,
		},
	})

	if err != nil {
		return fmt.Errorf("convert module %q chart values: %w", module.Spec.Module, err)
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
		return fmt.Errorf("get PlatformModule for pruning: %w", err)
	}

	current := sets.New[v1alpha1.ResourceRef]()
	for _, object := range request.Resources.All() {
		current.Insert(resourceRef(object))
	}

	for _, ref := range module.Status.Resources {
		if current.Has(ref) {
			continue
		}

		object, err := objectFromRef(ref)
		if err != nil {
			return fmt.Errorf("decode orphan resource %q: %w", ref.Name, err)
		}
		if retainedResource(object.GroupVersionKind()) {
			continue
		}

		err = request.Client.Delete(ctx, object, client.PropagationPolicy(metav1.DeletePropagationForeground))
		switch {
		case apierrors.IsNotFound(err):
			continue
		case meta.IsNoMatchError(err):
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
		return fmt.Errorf("get PlatformModule for recording: %w", err)
	}

	current := sets.New[v1alpha1.ResourceRef]()
	for _, object := range request.Resources.All() {
		current.Insert(resourceRef(object))
	}

	for _, ref := range module.Status.Resources {
		if current.Has(ref) {
			continue
		}

		object, err := objectFromRef(ref)
		if err != nil {
			return fmt.Errorf("decode recorded resource %q: %w", ref.Name, err)
		}
		if retainedResource(object.GroupVersionKind()) {
			current.Insert(ref)
		}
	}

	refs := current.UnsortedList()
	slices.SortFunc(refs, func(left v1alpha1.ResourceRef, right v1alpha1.ResourceRef) int {
		return cmp.Or(
			cmp.Compare(left.APIVersion, right.APIVersion),
			cmp.Compare(left.Kind, right.Kind),
			cmp.Compare(left.Namespace, right.Namespace),
			cmp.Compare(left.Name, right.Name),
		)
	})

	module.Status.Resources = refs

	return nil
}

func (c *Controller) cleanup(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return fmt.Errorf("get PlatformModule for cleanup: %w", err)
	}

	err = c.requireModuleCRRemoved(ctx, module)
	if err != nil {
		return fmt.Errorf("wait for module CR removal: %w", err)
	}

	err = c.deleteRecordedResources(ctx, module.Status.Resources)
	if err != nil {
		return fmt.Errorf("delete recorded resources: %w", err)
	}

	err = c.checkRecordedResourcesRemoved(ctx, module.Status.Resources)
	if err != nil {
		return fmt.Errorf("check recorded resource removal: %w", err)
	}

	return nil
}

var _ platformapi.PlatformObject = (*v1alpha1.PlatformModule)(nil)
