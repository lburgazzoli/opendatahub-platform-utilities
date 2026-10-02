package serving

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
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
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	platformpredicate "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var (
	ErrInvalidServingConfig    = errors.New("invalid Serving controller configuration")
	ErrInvalidServingState     = errors.New("invalid Serving management state")
	ErrInvalidStatusProjection = errors.New("invalid Serving status projection")
)

type ServingController struct {
	client    client.Client
	reader    client.Reader
	registry  *modules.Registry
	renderers map[string]*manifestengine.Engine
	deployer  *deploy.Action
}

func Setup(manager manager.Manager, registry *modules.Registry) error {
	if registry == nil {
		return fmt.Errorf("%w: registry is required", ErrInvalidServingConfig)
	}

	for _, name := range []string{"kserve", "aigateway"} {
		if _, found := registry.Get(name); !found {
			return fmt.Errorf("%w: module %q is required", ErrInvalidServingConfig, name)
		}
	}

	controller := &ServingController{
		client:    manager.GetClient(),
		reader:    manager.GetAPIReader(),
		registry:  registry,
		renderers: make(map[string]*manifestengine.Engine),
		deployer:  deploy.New(deploy.WithFieldOwner("example-serving")),
	}
	for _, name := range registry.Names() {
		definition, _ := registry.Get(name)
		renderer, err := helm.NewEngine(
			helm.Source{
				Chart:               definition.Chart,
				ReleaseName:         cmp.Or(definition.Config.Spec.Chart.Name, name),
				ReleaseVersion:      definition.Config.Spec.Chart.Version,
				ProcessDependencies: true,
			},
			helm.WithCache(),
		)
		if err != nil {
			return fmt.Errorf("create %s projection renderer: %w", name, err)
		}

		controller.renderers[name] = renderer
	}

	statusWatch := platformpredicate.Dependent(platformpredicate.DependentOptions{
		WatchDelete: true,
		WatchUpdate: true,
		WatchStatus: true,
	})
	kserve, _ := registry.Get("kserve")
	aigateway, _ := registry.Get("aigateway")

	return reconciler.For(manager, v1alpha1.NewServing(), reconciler.WithControllerName("example-serving")).
		Owns(v1alpha1.NewPlatform()).
		OwnsGVK(kserve.GVK(),
			reconciler.WithPredicates(statusWatch),
			reconciler.Dynamic(reconciler.CrdExists(kserve.GVK())),
		).
		OwnsGVK(aigateway.GVK(),
			reconciler.WithPredicates(statusWatch),
			reconciler.Dynamic(reconciler.CrdExists(aigateway.GVK())),
		).
		WithActionFunc(controller.reconcile, pipeline.WithName("project-serving")).
		Build()
}

func (c *ServingController) reconcile(ctx context.Context, request *pipeline.Request) error {
	serving, err := reconciler.Instance[*v1alpha1.Serving](request)
	if err != nil {
		return err
	}
	// The framework owns conditions and observedGeneration. Projection status
	// is applied under a separate field owner and must be absent from its SSA.
	defer func() {
		serving.Status.Kserve = nil
		serving.Status.MaaS = nil
	}()

	selected, pending, err := c.selectModules(ctx, serving)
	if err != nil {
		return err
	}
	if pending {
		return action.NewError("waiting for module CR removal").Advisory().WithRequeueAfter(2 * time.Second)
	}

	objects, err := c.renderResources(ctx, serving, selected)
	if err != nil {
		return err
	}

	_, err = c.deployer.Run(ctx, deploy.RunOptions{
		Client:    c.client,
		Owner:     serving,
		Resources: resources.New(objects),
	})
	if err != nil {
		return fmt.Errorf("deploy Serving projections: %w", err)
	}

	err = c.projectStatus(ctx, serving, selected)
	if err != nil {
		return err
	}

	return nil
}

func (c *ServingController) selectModules(ctx context.Context, serving *v1alpha1.Serving) ([]string, bool, error) {
	states := []struct {
		name  string
		state platformapi.ManagementState
	}{
		{name: "kserve", state: serving.Spec.Kserve.ManagementState},
		{name: "aigateway", state: serving.Spec.MaaS.ManagementState},
	}

	selected := make([]string, 0, len(states))
	for _, item := range states {
		switch item.state {
		case platformapi.ManagementStateManaged:
			selected = append(selected, item.name)
		case platformapi.ManagementStateRemoved, "":
			absent, removeErr := c.removeModuleCR(ctx, item.name)
			if removeErr != nil {
				return nil, false, removeErr
			}
			if !absent {
				return nil, true, nil
			}
		default:
			return nil, false, fmt.Errorf("%w: %s is %q", ErrInvalidServingState, item.name, item.state)
		}
	}
	return selected, false, nil
}

func (c *ServingController) renderResources(
	ctx context.Context,
	serving *v1alpha1.Serving,
	selected []string,
) (resources.List, error) {
	objects := make(resources.List, 0, len(selected)+1)
	platform := v1alpha1.NewPlatform()
	platform.Name = v1alpha1.PlatformName
	platform.Spec.Modules = selected

	platformObject, err := resources.ToUnstructured(platform)
	if err != nil {
		return nil, fmt.Errorf("convert Platform: %w", err)
	}
	objects = append(objects, *platformObject)

	source, err := resources.ToUnstructured(serving)
	if err != nil {
		return nil, fmt.Errorf("convert Serving source: %w", err)
	}
	source.SetGroupVersionKind(v1alpha1.ServingGVK)

	for _, name := range selected {
		definition, _ := c.registry.Get(name)
		target := map[string]any{
			"object": map[string]any{
				"apiVersion": definition.Config.Spec.ModuleRef.APIVersion,
				"kind":       definition.Config.Spec.ModuleRef.Kind,
				"metadata":   map[string]any{"name": definition.Config.Spec.ModuleRef.Name},
			},
		}

		rendered, renderErr := c.renderProjection(ctx, name, *source, true, false, target)
		if renderErr != nil {
			return nil, fmt.Errorf("project %s spec: %w", name, renderErr)
		}

		if len(rendered) != 1 || rendered[0].GroupVersionKind() != definition.GVK() ||
			rendered[0].GetName() != definition.Config.Spec.ModuleRef.Name {
			return nil, fmt.Errorf("%w: %s spec rendered an unexpected object", ErrInvalidStatusProjection, name)
		}

		objects = append(objects, rendered...)
	}

	return objects, nil
}

func (c *ServingController) projectStatus(
	ctx context.Context,
	serving *v1alpha1.Serving,
	selected []string,
) error {
	for _, name := range c.registry.Names() {
		definition, _ := c.registry.Get(name)
		source := &unstructured.Unstructured{Object: make(map[string]any)}
		source.SetGroupVersionKind(definition.GVK())
		source.SetName(definition.Config.Spec.ModuleRef.Name)

		exists := false
		ready := false
		if slices.Contains(selected, name) {
			module, found, moduleReady, err := c.moduleSource(ctx, name)
			if err != nil {
				return err
			}

			source = module
			exists = found
			ready = moduleReady
		}

		state := platformapi.ManagementStateRemoved
		if slices.Contains(selected, name) {
			state = platformapi.ManagementStateManaged
		}

		target := map[string]any{
			"object": map[string]any{
				"apiVersion": v1alpha1.GroupVersion.String(),
				"kind":       v1alpha1.ServingGVK.Kind,
				"metadata":   map[string]any{"name": serving.Name},
			},
			"managementState": string(state),
		}

		rendered, err := c.renderProjection(ctx, name, *source, exists, ready, target)
		if err != nil {
			return fmt.Errorf("project %s status: %w", name, err)
		}

		if len(rendered) != 1 || rendered[0].GroupVersionKind() != v1alpha1.ServingGVK ||
			rendered[0].GetName() != serving.Name {
			return fmt.Errorf("%w: %s status rendered an unexpected object", ErrInvalidStatusProjection, name)
		}

		err = resources.ApplyStatus(ctx, c.client, &rendered[0], client.FieldOwner("example-serving-status-"+name))
		if err != nil {
			return fmt.Errorf("apply %s Serving status: %w", name, err)
		}
	}

	return nil
}

func (c *ServingController) renderProjection(
	ctx context.Context,
	name string,
	source unstructured.Unstructured,
	exists bool,
	ready bool,
	target map[string]any,
) (resources.List, error) {
	values := manifesttypes.Values{
		"module": map[string]any{"enabled": false},
		"projections": map[string]any{
			"enabled": true,
			"input":   source.Object,
			"exists":  exists,
			"ready":   ready,
			"targets": []any{target},
		},
	}

	return c.renderers[name].Render(ctx, manifestrender.WithValues(values))
}

func (c *ServingController) removeModuleCR(ctx context.Context, name string) (bool, error) {
	definition, _ := c.registry.Get(name)
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(definition.GVK())
	key := types.NamespacedName{Name: definition.Config.Spec.ModuleRef.Name}

	err := c.reader.Get(ctx, key, object)
	switch {
	case apierrors.IsNotFound(err):
		return true, nil
	case meta.IsNoMatchError(err):
		return true, nil
	case err != nil:
		return false, fmt.Errorf("get %s CR: %w", name, err)
	}

	if object.GetDeletionTimestamp().IsZero() {
		err = c.client.Delete(ctx, object)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("delete %s CR: %w", name, err)
		}
	}

	return false, nil
}

func (c *ServingController) moduleSource(
	ctx context.Context,
	name string,
) (*unstructured.Unstructured, bool, bool, error) {
	definition, _ := c.registry.Get(name)
	synthetic := &unstructured.Unstructured{Object: make(map[string]any)}
	synthetic.SetGroupVersionKind(definition.GVK())
	synthetic.SetName(definition.Config.Spec.ModuleRef.Name)

	value, err := c.client.Scheme().New(definition.GVK())
	if err != nil {
		return nil, false, false, fmt.Errorf("construct %s object: %w", name, err)
	}
	object, ok := value.(platformapi.PlatformObject)
	if !ok {
		return nil, false, false, fmt.Errorf(
			"%w: %s does not implement PlatformObject", ErrInvalidStatusProjection, definition.GVK(),
		)
	}

	err = c.reader.Get(ctx, types.NamespacedName{Name: definition.Config.Spec.ModuleRef.Name}, object)
	switch {
	case apierrors.IsNotFound(err):
		return synthetic, false, false, nil
	case meta.IsNoMatchError(err):
		return synthetic, false, false, nil
	case err != nil:
		return nil, false, false, fmt.Errorf("get %s status: %w", name, err)
	}

	source, err := resources.ToUnstructured(object)
	if err != nil {
		return nil, false, false, fmt.Errorf("convert %s source: %w", name, err)
	}
	source.SetGroupVersionKind(definition.GVK())

	ready := condition.IsTrue(object.GetStatus(), string(platformapi.ConditionTypeReady)) &&
		object.GetStatus().ObservedGeneration == object.GetGeneration()

	return source, true, ready, nil
}
