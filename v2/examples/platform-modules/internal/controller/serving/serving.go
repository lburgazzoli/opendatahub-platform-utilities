package serving

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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
	client         client.Client
	reader         client.Reader
	registry       *modules.Registry
	specRenderer   *manifestengine.Engine
	statusRenderer *manifestengine.Engine
	deployer       *deploy.Action
}

func Setup(manager manager.Manager, registry *modules.Registry, chartDir string) error {
	if registry == nil || chartDir == "" {
		return fmt.Errorf("%w: registry and chart directory are required", ErrInvalidServingConfig)
	}

	for _, name := range []string{"kserve", "aigateway"} {
		if _, found := registry.Get(name); !found {
			return fmt.Errorf("%w: module %q is required", ErrInvalidServingConfig, name)
		}
	}

	specRenderer, err := helm.NewEngine(
		helm.Source{Chart: filepath.Join(chartDir, "module-spec"), ReleaseName: "module-spec"},
		helm.WithCache(),
	)
	if err != nil {
		return fmt.Errorf("create spec projection renderer: %w", err)
	}

	statusRenderer, err := helm.NewEngine(
		helm.Source{Chart: filepath.Join(chartDir, "serving-status"), ReleaseName: "serving-status"},
		helm.WithCache(),
	)
	if err != nil {
		return fmt.Errorf("create status projection renderer: %w", err)
	}

	controller := &ServingController{
		client:         manager.GetClient(),
		reader:         manager.GetAPIReader(),
		registry:       registry,
		specRenderer:   specRenderer,
		statusRenderer: statusRenderer,
		deployer:       deploy.New(deploy.WithFieldOwner("example-serving")),
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

	objects, err := c.renderResources(ctx, selected)
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

	err = c.projectStatus(ctx, selected)
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

func (c *ServingController) renderResources(ctx context.Context, selected []string) (resources.List, error) {
	objects := make(resources.List, 0, len(selected)+1)
	platform := v1alpha1.NewPlatform()
	platform.Name = v1alpha1.PlatformName
	platform.Spec.Modules = selected

	platformObject, err := resources.ToUnstructured(platform)
	if err != nil {
		return nil, fmt.Errorf("convert Platform: %w", err)
	}
	objects = append(objects, *platformObject)

	for _, name := range selected {
		definition, _ := c.registry.Get(name)
		rendered, renderErr := c.specRenderer.Render(ctx, manifestrender.WithValues(manifesttypes.Values{
			"apiVersion":      definition.APIVersion,
			"kind":            definition.Kind,
			"managementState": string(platformapi.ManagementStateManaged),
		}))
		if renderErr != nil {
			return nil, fmt.Errorf("project %s spec: %w", name, renderErr)
		}
		objects = append(objects, rendered...)
	}
	return objects, nil
}

func (c *ServingController) projectStatus(ctx context.Context, selected []string) error {
	ready := make(map[string]bool, len(selected))
	for _, name := range c.registry.Names() {
		if !slices.Contains(selected, name) {
			continue
		}

		value, err := c.moduleReady(ctx, name)
		if err != nil {
			return err
		}
		ready[name] = value
	}

	rendered, err := c.statusRenderer.Render(ctx, manifestrender.WithValues(manifesttypes.Values{
		"kserveReady": ready["kserve"],
		"maasReady":   ready["aigateway"],
	}))
	if err != nil {
		return fmt.Errorf("project Serving status: %w", err)
	}
	if len(rendered) != 1 {
		return fmt.Errorf("%w: chart rendered %d objects", ErrInvalidStatusProjection, len(rendered))
	}

	err = resources.ApplyStatus(ctx, c.client, &rendered[0], client.FieldOwner("example-serving-status"))
	if err != nil {
		return fmt.Errorf("apply Serving status: %w", err)
	}

	return nil
}

func (c *ServingController) removeModuleCR(ctx context.Context, name string) (bool, error) {
	definition, _ := c.registry.Get(name)
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(definition.GVK())
	key := types.NamespacedName{Name: v1alpha1.InstanceName}

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

func (c *ServingController) moduleReady(ctx context.Context, name string) (bool, error) {
	definition, _ := c.registry.Get(name)
	value, err := c.client.Scheme().New(definition.GVK())
	if err != nil {
		return false, fmt.Errorf("construct %s object: %w", name, err)
	}
	object, ok := value.(platformapi.PlatformObject)
	if !ok {
		return false, fmt.Errorf("%w: %s does not implement PlatformObject", ErrInvalidStatusProjection, definition.GVK())
	}

	err = c.reader.Get(ctx, types.NamespacedName{Name: v1alpha1.InstanceName}, object)
	switch {
	case apierrors.IsNotFound(err):
		return false, nil
	case meta.IsNoMatchError(err):
		return false, nil
	case err != nil:
		return false, fmt.Errorf("get %s status: %w", name, err)
	}

	return condition.IsTrue(object.GetStatus(), string(platformapi.ConditionTypeReady)) &&
		object.GetStatus().ObservedGeneration == object.GetGeneration(), nil
}
