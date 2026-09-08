package controller

import (
	"testing"
	"time"

	"github.com/onsi/gomega"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-plain/api/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestReconcileSkipsDeletingComponent(t *testing.T) {
	t.Parallel()

	g := gomega.NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(v1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())

	component := v1alpha1.NewHelmComponent()
	component.Namespace = "default"
	component.Name = "component"
	component.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	component.Finalizers = []string{"examples.odh.io/helmcomponent"}
	componentClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(component).
		Build()
	reconcilerValue := &HelmComponentReconciler{
		client:   componentClient,
		renderer: nil,
	}

	result, err := reconcilerValue.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: client.ObjectKeyFromObject(component),
	})

	g.Expect(err).NotTo(gomega.HaveOccurred())
	//nolint:exhaustruct_v5 // an empty result is the controller-runtime no-op result.
	g.Expect(result).To(gomega.Equal(ctrl.Result{}))
	objects := new(corev1.ConfigMapList)
	g.Expect(componentClient.List(t.Context(), objects)).To(gomega.Succeed())
	g.Expect(objects.Items).To(gomega.BeEmpty())
}

func TestUpdateStatusMatchesPipelineOutcomeSemantics(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		outcome       action.ActionError
		conditionTrue bool
	}{
		{
			name:          "success",
			outcome:       action.ActionError{},
			conditionTrue: true,
		},
		{
			name:          "terminal error",
			outcome:       action.NewError("render failed"),
			conditionTrue: false,
		},
	}

	for _, current := range tests {
		t.Run(current.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(v1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
			component := v1alpha1.NewHelmComponent()
			component.Namespace = "default"
			component.Name = "component"
			component.Generation = 4
			componentClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(component).
				WithObjects(component).
				Build()
			reconcilerValue := &HelmComponentReconciler{
				client:   componentClient,
				renderer: nil,
			}

			err := reconcilerValue.updateStatus(t.Context(), component, current.outcome)

			g.Expect(err).NotTo(gomega.HaveOccurred())
			g.Expect(component.Status.ObservedGeneration).To(gomega.Equal(int64(4)))
			g.Expect(condition.IsTrue(
				component,
				string(platformapi.ConditionTypeProvisioningSucceeded),
			)).To(gomega.Equal(current.conditionTrue))
			g.Expect(condition.Find(
				component,
				string(platformapi.ConditionTypeReady),
			)).NotTo(gomega.BeNil())
			g.Expect(condition.IsTrue(
				component,
				string(platformapi.ConditionTypeReady),
			)).To(gomega.Equal(current.conditionTrue))
		})
	}
}
