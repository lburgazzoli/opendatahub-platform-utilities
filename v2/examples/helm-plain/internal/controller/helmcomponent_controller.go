package controller

import (
	"context"
	"fmt"
	"slices"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-plain/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// HelmComponentReconciler renders and deploys the example Helm chart.
//
// +kubebuilder:rbac:groups=examples.odh.io,resources=helmcomponents,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=examples.odh.io,resources=helmcomponents/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch;delete
type HelmComponentReconciler struct {
	client   client.Client
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
		client:   manager.GetClient(),
		renderer: renderer,
	}
	ownedConfigMap := new(unstructured.Unstructured)
	ownedConfigMap.SetGroupVersionKind(kubegvk.ConfigMap)

	return ctrl.NewControllerManagedBy(manager).
		Named("helm-example").
		For(v1alpha1.NewHelmComponent()).
		Owns(ownedConfigMap).
		Complete(reconcilerValue)
}

func (r *HelmComponentReconciler) Reconcile(
	ctx context.Context,
	request ctrl.Request,
) (ctrl.Result, error) {
	component := v1alpha1.NewHelmComponent()
	err := r.client.Get(ctx, request.NamespacedName, component)
	switch {
	case apierrors.IsNotFound(err):
		//nolint:exhaustruct_v5 // an empty result is the controller-runtime no-op result.
		return ctrl.Result{}, nil
	case err != nil:
		return ctrl.Result{}, fmt.Errorf("load Helm component: %w", err)
	}

	if !component.GetDeletionTimestamp().IsZero() {
		//nolint:exhaustruct_v5 // an empty result is the controller-runtime no-op result.
		return ctrl.Result{}, nil
	}

	outcome := r.reconcileComponent(ctx, component)

	return r.finish(ctx, component, outcome)
}

func (r *HelmComponentReconciler) reconcileComponent(
	ctx context.Context,
	component *v1alpha1.HelmComponent,
) action.ActionError {
	var outcome action.ActionError

	collection := resources.New(nil)
	err := r.render(ctx, component, collection)
	if err != nil {
		outcome, _ = outcome.Add("render", err)
		return outcome
	}

	_, err = deploy.New(deploy.WithFieldOwner("helm-example")).Run(ctx, deploy.RunOptions{
		Client:    r.client,
		Owner:     component,
		Resources: collection,
	})
	if err != nil {
		outcome, _ = outcome.Add("deploy", err)
	}

	return outcome
}

func (r *HelmComponentReconciler) finish(
	ctx context.Context,
	component *v1alpha1.HelmComponent,
	outcome action.ActionError,
) (ctrl.Result, error) {
	statusErr := r.updateStatus(ctx, component, outcome)
	switch {
	case statusErr != nil:
		return ctrl.Result{}, statusErr
	case outcome.RequeueAfter() > 0:
		//nolint:exhaustruct_v5 // only the action's requeue delay is set.
		return ctrl.Result{RequeueAfter: outcome.RequeueAfter()}, nil
	case outcome.Err() != nil && outcome.Type() != action.ErrorTypeAdvisory:
		return ctrl.Result{}, fmt.Errorf("reconcile failed: %w", outcome.Err())
	default:
		//nolint:exhaustruct_v5 // an empty result is the controller-runtime no-op result.
		return ctrl.Result{}, nil
	}
}

func (r *HelmComponentReconciler) updateStatus(
	ctx context.Context,
	component *v1alpha1.HelmComponent,
	outcome action.ActionError,
) error {
	generation := component.GetGeneration()
	component.Status.ObservedGeneration = generation

	markOptions := []condition.MarkOption{condition.WithObservedGeneration(generation)}
	switch {
	case outcome.Err() == nil:
		condition.MarkTrue(component, string(platformapi.ConditionTypeProvisioningSucceeded), markOptions...)
	case outcome.Type() == action.ErrorTypeAdvisory:
		markOptions = append(markOptions,
			condition.WithReason("Advisory"),
			condition.WithMessage("%s", outcome.Error()),
		)
		condition.MarkTrue(component, string(platformapi.ConditionTypeProvisioningSucceeded), markOptions...)
	default:
		condition.MarkFalse(
			component,
			string(platformapi.ConditionTypeProvisioningSucceeded),
			append(markOptions, condition.WithError(outcome))...,
		)
	}

	aggregateConditions(component)

	return resources.ApplyStatus(
		ctx,
		r.client,
		component,
		client.FieldOwner("helm-example"),
		client.ForceOwnership,
	)
}

func aggregateConditions(component *v1alpha1.HelmComponent) {
	dependentTypes := make([]string, 0, len(component.GetConditions()))
	for _, current := range component.GetConditions() {
		if current.Type == string(platformapi.ConditionTypeReady) {
			continue
		}

		dependentTypes = append(dependentTypes, current.Type)
	}

	condition.Aggregate(
		component,
		string(platformapi.ConditionTypeReady),
		slices.Compact(dependentTypes)...,
	)
}

func (r *HelmComponentReconciler) render(
	ctx context.Context,
	component *v1alpha1.HelmComponent,
	collection resources.Accessor,
) error {
	rendered, err := r.renderer.Render(ctx, manifestengine.WithValues(map[string]any{
		"name": component.GetName(),
	}))
	if err != nil {
		return fmt.Errorf("render Helm chart: %w", err)
	}

	collection.Set(resources.List(rendered))

	return nil
}
