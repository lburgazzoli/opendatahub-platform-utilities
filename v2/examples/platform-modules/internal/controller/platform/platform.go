package platform

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"time"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/gc"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	ErrInvalidPlatformConfig = errors.New("invalid Platform controller configuration")
	ErrUnknownModule         = errors.New("unknown module")
	ErrInvalidPlatformModule = errors.New("invalid PlatformModule")
)

// PlatformController renders the selected PlatformModule resources.
type PlatformController struct {
	registry *modules.Registry
}

type PlatformModuleController struct {
	registry  *modules.Registry
	renderers map[string]*manifestengine.Engine
	reader    client.Reader
	image     string
	namespace string
}

func Setup(manager manager.Manager, registry *modules.Registry, image string, namespace string) error {
	if registry == nil || image == "" || namespace == "" {
		return fmt.Errorf("%w: registry, image, and namespace are required", ErrInvalidPlatformConfig)
	}

	platform := &PlatformController{registry: registry}
	err := reconciler.For(manager, v1alpha1.NewPlatform(),
		reconciler.WithControllerName("example-platform"),
	).
		Owns(v1alpha1.NewPlatformModule()).
		WithActionFunc(platform.render, pipeline.WithName("select-modules")).
		WithAction(deploy.New()).
		WithAction(gc.New(gc.StaticDiscovery(v1alpha1.PlatformModuleGVK))).
		Build()
	if err != nil {
		return fmt.Errorf("register Platform controller: %w", err)
	}

	module := &PlatformModuleController{
		registry:  registry,
		renderers: make(map[string]*manifestengine.Engine),
		reader:    manager.GetAPIReader(),
		image:     image,
		namespace: namespace,
	}
	for _, name := range registry.Names() {
		definition, _ := registry.Get(name)

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

		module.renderers[name] = renderer
	}

	return reconciler.For(manager, v1alpha1.NewPlatformModule(),
		reconciler.WithControllerName("example-platform-module"),
		reconciler.WithCleanupTimeout(0),
	).
		Owns(&appsv1.Deployment{}).
		WithActionFunc(module.render, pipeline.WithName("render-module-controller")).
		WithAction(deploy.New()).
		WithCleanupActionFunc(module.waitForModuleCR, pipeline.WithName("wait-for-module-cr")).
		Build()
}

func (c *PlatformController) render(ctx context.Context, request *pipeline.Request) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

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

func (c *PlatformModuleController) render(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return err
	}

	definition, found := c.registry.Get(module.Spec.Module)
	if !found || module.Name != definition.Config.Metadata.Name {
		return fmt.Errorf("%w: %q names module %q", ErrInvalidPlatformModule, module.Name, module.Spec.Module)
	}

	moduleValues, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&definition.Config.Spec)
	if err != nil {
		return fmt.Errorf("convert module %q spec: %w", module.Spec.Module, err)
	}
	moduleValues["enabled"] = true
	moduleValues["namespace"] = c.namespace
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

func (c *PlatformModuleController) waitForModuleCR(ctx context.Context, request *pipeline.Request) error {
	module, err := reconciler.Instance[*v1alpha1.PlatformModule](request)
	if err != nil {
		return err
	}

	definition, found := c.registry.Get(module.Spec.Module)
	if !found {
		return fmt.Errorf("%w during cleanup: %q", ErrUnknownModule, module.Spec.Module)
	}

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(definition.GVK())
	err = c.reader.Get(ctx, types.NamespacedName{Name: definition.Config.Spec.ModuleRef.Name}, object)
	switch {
	case err == nil:
		return action.NewErrorf("waiting for %s/%s removal", definition.GVK().Kind, object.GetName()).
			Advisory().WithRequeueAfter(2 * time.Second)
	case apierrors.IsNotFound(err):
		return nil
	default:
		return fmt.Errorf("get module CR %q: %w", definition.CRDName, err)
	}
}

var _ platformapi.PlatformObject = (*v1alpha1.Platform)(nil)
var _ platformapi.PlatformObject = (*v1alpha1.PlatformModule)(nil)
