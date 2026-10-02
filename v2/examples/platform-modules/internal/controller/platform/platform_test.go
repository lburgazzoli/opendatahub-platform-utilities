package platform

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/lburgazzoli/gomega-matchers/pkg/matchers/jq"
	"github.com/onsi/gomega"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestRenderSelectedModules(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	registry, err := modules.Load(filepath.Join("..", "..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())

	platform := v1alpha1.NewPlatform()
	platform.Spec.Modules = []string{"aigateway", "kserve"}
	request := &pipeline.Request{Instance: platform, Resources: resources.New(nil)}
	controller := &PlatformController{registry: registry}

	g.Expect(controller.render(t.Context(), request)).To(gomega.Succeed())
	g.Expect(request.Resources.Len()).To(gomega.Equal(2))

	names := make([]string, 0, request.Resources.Len())
	for _, object := range request.Resources.All() {
		g.Expect(object.Object).To(jq.Matchf(
			`.apiVersion == %q and .kind == %q and .spec.module == .metadata.name`,
			v1alpha1.GroupVersion.String(),
			v1alpha1.PlatformModuleGVK.Kind,
		))
		names = append(names, object.GetName())
	}
	g.Expect(names).To(gomega.ConsistOf("aigateway", "kserve"))

	platform.Spec.Modules = nil
	g.Expect(controller.render(t.Context(), request)).To(gomega.Succeed())
	g.Expect(request.Resources.Len()).To(gomega.BeZero())
}

func TestRenderRejectsUnknownModuleWithoutPublishing(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	registry, err := modules.Load(filepath.Join("..", "..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())

	platform := v1alpha1.NewPlatform()
	platform.Spec.Modules = []string{"kserve", "missing"}
	request := &pipeline.Request{Instance: platform, Resources: resources.New(nil)}
	controller := &PlatformController{registry: registry}

	err = controller.render(t.Context(), request)
	g.Expect(errors.Is(err, ErrUnknownModule)).To(gomega.BeTrue())
	g.Expect(request.Resources.Len()).To(gomega.BeZero())
}

func TestPruneModulesDeletesUnselectedModules(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(v1alpha1.AddToScheme(scheme)).To(gomega.Succeed())

	selected := v1alpha1.NewPlatformModule()
	selected.Name = "kserve"
	selected.Spec.Module = "kserve"
	stale := v1alpha1.NewPlatformModule()
	stale.Name = "aigateway"
	stale.Spec.Module = "aigateway"
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(selected, stale).Build()

	platform := v1alpha1.NewPlatform()
	platform.Spec.Modules = []string{selected.Name}
	request := &pipeline.Request{Client: kubeClient, Instance: platform}
	controller := new(PlatformController)

	g.Expect(controller.pruneModules(t.Context(), request)).To(gomega.Succeed())
	err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(selected), new(v1alpha1.PlatformModule))
	g.Expect(err).To(gomega.Succeed())
	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(stale), new(v1alpha1.PlatformModule))
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())

	platform.Spec.Modules = nil
	g.Expect(controller.pruneModules(t.Context(), request)).To(gomega.Succeed())
	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(selected), new(v1alpha1.PlatformModule))
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
}

func TestUpdateStatusAggregatesSelectedModules(t *testing.T) {
	t.Parallel()

	ready := v1alpha1.NewPlatformModule()
	ready.Name = "kserve"
	ready.Generation = 2
	ready.Status.ObservedGeneration = ready.Generation
	condition.MarkTrue(ready.GetStatus(), string(platformapi.ConditionTypeReady))

	unready := ready.DeepCopy()
	condition.MarkFalse(unready.GetStatus(), string(platformapi.ConditionTypeReady))

	stale := ready.DeepCopy()
	stale.Status.ObservedGeneration = ready.Generation - 1

	selected := []string{"kserve"}
	waiting := "modules not ready: kserve"
	missingMessage := "modules not ready: aigateway, kserve"
	tests := []struct {
		name        string
		wantMessage string
		selected    []string
		modules     []client.Object
		wantReady   bool
	}{
		{name: "no modules selected", wantReady: true},
		{name: "missing modules", selected: []string{"kserve", "aigateway"}, wantMessage: missingMessage},
		{name: "module not ready", selected: selected, modules: []client.Object{unready.DeepCopy()}, wantMessage: waiting},
		{name: "stale status", selected: selected, modules: []client.Object{stale.DeepCopy()}, wantMessage: waiting},
		{name: "selected module ready", selected: selected, modules: []client.Object{ready.DeepCopy()}, wantReady: true},
		{name: "unselected module ignored", modules: []client.Object{unready.DeepCopy()}, wantReady: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)

			scheme := runtime.NewScheme()
			g.Expect(v1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
			kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tt.modules...).Build()

			platform := v1alpha1.NewPlatform()
			platform.Generation = 3
			platform.Spec.Modules = tt.selected
			request := &pipeline.Request{Client: kubeClient, Instance: platform}
			controller := new(PlatformController)

			g.Expect(controller.updateStatus(t.Context(), request)).To(gomega.Succeed())

			status := condition.Find(platform.GetStatus(), string(v1alpha1.ConditionModulesReady))
			g.Expect(status).NotTo(gomega.BeNil())
			wantStatus := metav1.ConditionFalse
			if tt.wantReady {
				wantStatus = metav1.ConditionTrue
			}
			g.Expect(status.Status).To(gomega.Equal(wantStatus))
			g.Expect(status.Message).To(gomega.Equal(tt.wantMessage))
			g.Expect(status.ObservedGeneration).To(gomega.Equal(platform.Generation))
		})
	}
}
