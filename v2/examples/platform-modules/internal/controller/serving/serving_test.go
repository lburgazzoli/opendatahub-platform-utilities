package serving_test

import (
	"path/filepath"
	"testing"

	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	"github.com/onsi/gomega"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestModuleSpecProjections(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	registry, err := modules.Load(filepath.Join("..", "..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())

	for _, name := range registry.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			definition, _ := registry.Get(name)
			renderer, err := helm.NewEngine(helm.Source{
				Chart:               definition.Chart,
				ReleaseName:         definition.Config.Spec.Chart.Name,
				ProcessDependencies: true,
			})
			g.Expect(err).To(gomega.Succeed())

			serving := v1alpha1.NewServing()
			serving.Name = "cluster"
			serving.Spec.Kserve.ManagementState = "Managed"
			serving.Spec.MaaS.ManagementState = "Managed"
			source, err := resources.ToUnstructured(serving)
			g.Expect(err).To(gomega.Succeed())
			source.SetGroupVersionKind(v1alpha1.ServingGVK)

			target := map[string]any{"object": map[string]any{
				"apiVersion": definition.Config.Spec.ModuleRef.APIVersion,
				"kind":       definition.Config.Spec.ModuleRef.Kind,
				"metadata":   map[string]any{"name": definition.Config.Spec.ModuleRef.Name},
			}}
			objects, err := renderer.Render(t.Context(), manifestrender.WithValues(manifesttypes.Values{
				"module": map[string]any{"enabled": false},
				"projections": map[string]any{
					"enabled": true,
					"input":   source.Object,
					"targets": []any{target},
				},
			}))
			g.Expect(err).To(gomega.Succeed())
			g.Expect(objects).To(gomega.HaveLen(1))
			g.Expect(objects[0].GroupVersionKind()).To(gomega.Equal(definition.GVK()))
			g.Expect(objects[0].GetName()).To(gomega.Equal(definition.Config.Spec.ModuleRef.Name))
			g.Expect(objects[0].Object).To(gomega.HaveKey("spec"))
			g.Expect(objects[0].Object).NotTo(gomega.HaveKey("status"))
		})
	}
}

func TestModuleStatusProjections(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	registry, err := modules.Load(filepath.Join("..", "..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())

	for _, name := range registry.Names() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)
			definition, _ := registry.Get(name)
			renderer, err := helm.NewEngine(helm.Source{
				Chart:               definition.Chart,
				ReleaseName:         definition.Config.Spec.Chart.Name,
				ProcessDependencies: true,
			})
			g.Expect(err).To(gomega.Succeed())

			source := &unstructured.Unstructured{Object: make(map[string]any)}
			source.SetGroupVersionKind(definition.GVK())
			source.SetName(definition.Config.Spec.ModuleRef.Name)
			target := map[string]any{
				"object": map[string]any{
					"apiVersion": v1alpha1.GroupVersion.String(),
					"kind":       v1alpha1.ServingGVK.Kind,
					"metadata":   map[string]any{"name": "cluster"},
				},
				"managementState": "Managed",
			}
			objects, err := renderer.Render(t.Context(), manifestrender.WithValues(manifesttypes.Values{
				"module": map[string]any{"enabled": false},
				"projections": map[string]any{
					"enabled": true,
					"input":   source.Object,
					"exists":  true,
					"ready":   true,
					"targets": []any{target},
				},
			}))
			g.Expect(err).To(gomega.Succeed())
			g.Expect(objects).To(gomega.HaveLen(1))
			g.Expect(objects[0].GroupVersionKind()).To(gomega.Equal(v1alpha1.ServingGVK))
			g.Expect(objects[0].GetName()).To(gomega.Equal("cluster"))
			g.Expect(objects[0].Object).To(gomega.HaveKey("status"))
			g.Expect(objects[0].Object).NotTo(gomega.HaveKey("spec"))
		})
	}
}
