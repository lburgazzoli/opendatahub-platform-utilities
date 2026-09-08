package controller

import (
	"context"
	"fmt"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler"
	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// HelmComponentReconciler renders and deploys the example Helm chart.
//
// +kubebuilder:rbac:groups=examples.odh.io,resources=helmcomponents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=examples.odh.io,resources=helmcomponents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
type HelmComponentReconciler struct {
	renderer *manifestengine.Engine
}

func Setup(manager manager.Manager, chartPath string) error {
	renderer, err := helm.NewEngine(
		//nolint:exhaustruct_v5 // the example uses a chart path and release name.
		helm.Source{Chart: chartPath, ReleaseName: "component"},
		helm.WithCache(),
		helm.WithSourceAnnotations(true),
	)
	if err != nil {
		return fmt.Errorf("create Helm renderer: %w", err)
	}

	reconcilerValue := &HelmComponentReconciler{
		renderer: renderer,
	}
	ownedConfigMap := new(unstructured.Unstructured)
	ownedConfigMap.SetGroupVersionKind(kubegvk.ConfigMap)

	return reconciler.For(
		manager,
		v1alpha1.NewHelmComponent(),
		reconciler.WithControllerName("helm-example"),
		reconciler.WithFieldOwner("helm-example"),
	).
		Owns(ownedConfigMap).
		WithActionFunc(reconcilerValue.render, pipeline.WithName("render")).
		WithAction(deploy.New(deploy.WithFieldOwner("helm-example"))).
		Build()
}

func (r *HelmComponentReconciler) render(
	ctx context.Context,
	request *pipeline.Request,
) error {
	component, ok := request.Instance.(*v1alpha1.HelmComponent)
	if !ok {
		return fmt.Errorf(
			"%w: expected %T, got %T",
			v1alpha1.ErrRequestInstance,
			new(v1alpha1.HelmComponent),
			request.Instance,
		)
	}

	rendered, err := r.renderer.Render(ctx, manifestengine.WithValues(map[string]any{
		"name": component.GetName(),
	}))
	if err != nil {
		return fmt.Errorf("render Helm chart: %w", err)
	}

	request.Resources.Set(resources.List(rendered))

	return nil
}
