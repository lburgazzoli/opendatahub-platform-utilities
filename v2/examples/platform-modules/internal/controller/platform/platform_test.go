package platform_test

import (
	"path/filepath"
	"testing"

	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	"github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestModuleChartRunsSameImageWithSelectedCRD(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	registry, err := modules.Load(filepath.Join("..", "..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())

	for _, name := range registry.Names() {
		definition, _ := registry.Get(name)
		renderer, err := helm.NewEngine(helm.Source{
			Chart:               definition.Chart,
			ReleaseName:         definition.Config.Spec.Chart.Name,
			ReleaseNamespace:    "default",
			ReleaseVersion:      definition.Config.Spec.Chart.Version,
			ProcessDependencies: true,
		})
		g.Expect(err).To(gomega.Succeed())

		moduleValues, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&definition.Config.Spec)
		g.Expect(err).To(gomega.Succeed())
		moduleValues["config"] = map[string]any{"replicas": 2}
		moduleValues["enabled"] = true
		moduleValues["namespace"] = "default"
		moduleValues["image"] = "ttl.sh/example:24h"

		objects, err := renderer.Render(t.Context(), manifestrender.WithValues(manifesttypes.Values{
			"module":      moduleValues,
			"projections": map[string]any{"enabled": false},
		}))
		g.Expect(err).To(gomega.Succeed())
		g.Expect(objects).To(gomega.HaveLen(5))

		var deployment *unstructured.Unstructured
		var crd *unstructured.Unstructured
		for index := range objects {
			switch objects[index].GetKind() {
			case "Deployment":
				deployment = &objects[index]
			case "CustomResourceDefinition":
				crd = &objects[index]
			}
		}
		g.Expect(deployment).NotTo(gomega.BeNil())
		assertModuleCRD(t, crd, definition)

		containers, found, err := unstructured.NestedSlice(deployment.Object, "spec", "template", "spec", "containers")
		g.Expect(err).To(gomega.Succeed())
		g.Expect(found).To(gomega.BeTrue())
		container, ok := containers[0].(map[string]any)
		g.Expect(ok).To(gomega.BeTrue())
		g.Expect(container["image"]).To(gomega.Equal("ttl.sh/example:24h"))
		g.Expect(container["args"]).To(gomega.Equal([]any{"run", "module", name}))

		replicas, found, err := unstructured.NestedInt64(deployment.Object, "spec", "replicas")
		g.Expect(err).To(gomega.Succeed())
		g.Expect(found).To(gomega.BeTrue())
		g.Expect(replicas).To(gomega.Equal(int64(2)))
	}
}

func assertModuleCRD(t *testing.T, crd *unstructured.Unstructured, definition modules.Definition) {
	t.Helper()
	g := gomega.NewWithT(t)
	g.Expect(crd).NotTo(gomega.BeNil())
	g.Expect(crd.GetName()).To(gomega.Equal(definition.CRDName))

	group, found, err := unstructured.NestedString(crd.Object, "spec", "group")
	g.Expect(err).To(gomega.Succeed())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(group).To(gomega.Equal(definition.GVK().Group))

	kind, found, err := unstructured.NestedString(crd.Object, "spec", "names", "kind")
	g.Expect(err).To(gomega.Succeed())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(kind).To(gomega.Equal(definition.GVK().Kind))

	plural, found, err := unstructured.NestedString(crd.Object, "spec", "names", "plural")
	g.Expect(err).To(gomega.Succeed())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(plural).To(gomega.Equal(definition.Plural))

	singular, found, err := unstructured.NestedString(crd.Object, "spec", "names", "singular")
	g.Expect(err).To(gomega.Succeed())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(singular).To(gomega.Equal(definition.Config.Metadata.Name))

	versions, found, err := unstructured.NestedSlice(crd.Object, "spec", "versions")
	g.Expect(err).To(gomega.Succeed())
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(versions).To(gomega.HaveLen(1))
	g.Expect(versions[0]).To(gomega.HaveKeyWithValue("name", definition.GVK().Version))
}
