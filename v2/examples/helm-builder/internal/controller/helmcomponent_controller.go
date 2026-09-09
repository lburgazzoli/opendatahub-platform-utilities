package controller

import (
	"context"
	"fmt"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/api/v1alpha1"
	moduleconfig "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/config"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	kubeGVK "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const controllerName = "helm-example"

// HelmComponentReconciler renders and deploys the example Helm chart.
//
// +kubebuilder:rbac:groups=examples.odh.io,resources=helmcomponents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=examples.odh.io,resources=helmcomponents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch
type HelmComponentReconciler struct {
	renderer      *manifestengine.Engine
	configuration *moduleconfig.Config
}

func Setup(manager manager.Manager, configuration *moduleconfig.Config) error {
	renderer, err := helm.NewEngine(
		// the example uses a chart path and release name.
		//nolint:exhaustruct_v5
		helm.Source{
			Chart:       configuration.ChartPath,
			ReleaseName: "component",
		},
		helm.WithCache(),
		helm.WithSourceAnnotations(true),
	)
	if err != nil {
		return fmt.Errorf("create Helm renderer: %w", err)
	}

	r := &HelmComponentReconciler{
		renderer:      renderer,
		configuration: configuration,
	}

	return reconciler.For(
		manager,
		v1alpha1.NewHelmComponent(),
		reconciler.WithControllerName(controllerName),
	).
		Owns(&corev1.ConfigMap{}).
		WatchesGVK(kubeGVK.Route, reconciler.When(routeAPIAvailable())).
		WithActionFunc(r.render, pipeline.WithName("render")).
		WithAction(deploy.New()).
		Build()
}

func routeAPIAvailable() pipeline.Guard {
	return pipeline.NewGuard("route-api-available", func(_ context.Context, request *pipeline.Request) (bool, error) {
		if request == nil || request.Client == nil {
			return false, nil
		}

		_, err := request.Client.RESTMapper().RESTMapping(
			kubeGVK.Route.GroupKind(),
			kubeGVK.Route.Version,
		)
		switch {
		case err == nil:
			return true, nil
		case meta.IsNoMatchError(err):
			return false, nil
		default:
			return false, fmt.Errorf("check Route API availability: %w", err)
		}
	})
}

func (r *HelmComponentReconciler) render(
	ctx context.Context,
	request *pipeline.Request,
) error {
	component, err := reconciler.Instance[*v1alpha1.HelmComponent](request)
	if err != nil {
		return err
	}

	rendered, err := r.renderer.Render(ctx, manifestrender.WithValues(manifesttypes.Values{
		"name":      component.GetName(),
		"namespace": component.GetNamespace(),
	}))
	if err != nil {
		return fmt.Errorf("render Helm chart: %w", err)
	}

	request.Resources.Set(rendered)

	return nil
}
