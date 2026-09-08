package renderer_test

import (
	"os"
	"path/filepath"
	"testing"
	testingfs "testing/fstest"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	gotemplate "github.com/k8s-manifest-kit/renderer-gotemplate/pkg"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	kustomize "github.com/k8s-manifest-kit/renderer-kustomize/pkg"
	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/renderer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const helmChartYAML = `apiVersion: v2
name: fixture
version: 0.1.0
`

const helmTemplateYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.name }}-helm
data:
  source: helm
`

const kustomizationYAML = `resources:
- configmap.yaml
`

const kustomizeResourceYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: component-kustomize
data:
  source: kustomize
`

const updatedKustomizeResourceYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: updated-kustomize
data:
  source: changed
`

const templateYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .name }}-template
data:
  source: gotemplate
`

const invalidTemplateYAML = "{{ .missing"

func TestControllerRendersAndPublishesComposedSources(t *testing.T) {
	t.Parallel()

	engine, _ := newEngine(t)
	controller := renderer.NewController(engine, valuesForInstance)
	request := &pipeline.Request{
		Instance:  consumerInstance("component"),
		Resources: resources.New(nil),
	}

	err := controller.RenderResources(t.Context(), request)

	g := NewWithT(t)
	g.Expect(err).Should(Succeed())
	g.Expect(request.Resources.Len()).Should(Equal(3))
	g.Expect(resourceNames(request.Resources.Get())).Should(Equal([]string{
		"component-helm",
		"component-kustomize",
		"component-template",
	}))
	for _, object := range request.Resources.Get() {
		g.Expect(object.GetAnnotations()).Should(HaveKey(manifesttypes.AnnotationSourceType))
	}
}

func TestControllerRendersValuesFromCurrentInstance(t *testing.T) {
	t.Parallel()

	engine, _ := newEngine(t)
	controller := renderer.NewController(engine, valuesForInstance)
	request := &pipeline.Request{
		Instance:  consumerInstance("first"),
		Resources: resources.New(nil),
	}

	g := NewWithT(t)
	g.Expect(controller.RenderResources(t.Context(), request)).Should(Succeed())
	g.Expect(resourceNames(request.Resources.Get())).Should(Equal([]string{
		"first-helm",
		"component-kustomize",
		"first-template",
	}))

	request.Instance = consumerInstance("second")
	g.Expect(controller.RenderResources(t.Context(), request)).Should(Succeed())
	g.Expect(resourceNames(request.Resources.Get())).Should(Equal([]string{
		"second-helm",
		"component-kustomize",
		"second-template",
	}))
}

func TestControllerUsesManifestKitCacheOnSecondRender(t *testing.T) {
	t.Parallel()

	engine, kustomizePath := newEngine(t)
	controller := renderer.NewController(engine, valuesForInstance)
	request := &pipeline.Request{
		Instance:  consumerInstance("component"),
		Resources: resources.New(nil),
	}

	g := NewWithT(t)
	g.Expect(controller.RenderResources(t.Context(), request)).Should(Succeed())

	writeFile(t, filepath.Join(kustomizePath, "configmap.yaml"), updatedKustomizeResourceYAML)
	g.Expect(controller.RenderResources(t.Context(), request)).Should(Succeed())
	g.Expect(resourceNames(request.Resources.Get())).Should(Equal([]string{
		"component-helm",
		"component-kustomize",
		"component-template",
	}))
}

func TestControllerActionRunsThroughPipeline(t *testing.T) {
	t.Parallel()

	engine, _ := newEngine(t)
	controller := renderer.NewController(engine, valuesForInstance)
	request := &pipeline.Request{
		Instance:  consumerInstance("component"),
		Resources: resources.New(nil),
	}
	pipelineValue := pipeline.New().WithAction(controller.Action())

	err := pipelineValue.Run(t.Context(), request)

	g := NewWithT(t)
	g.Expect(err.Err()).Should(Succeed())
	g.Expect(request.Resources.Len()).Should(Equal(3))
}

func TestControllerDoesNotPublishOnRenderFailure(t *testing.T) {
	t.Parallel()

	invalidTemplateFS := testingfs.MapFS{
		"templates/configmap.yaml.tmpl": &testingfs.MapFile{
			Data: []byte(invalidTemplateYAML),
		},
	}
	templateRenderer, err := gotemplate.New(
		[]gotemplate.Source{{FS: invalidTemplateFS, Path: "templates/*.yaml.tmpl"}},
		gotemplate.WithCache(),
	)

	g := NewWithT(t)
	g.Expect(err).Should(Succeed())
	engine, err := manifestengine.New(manifestengine.WithRenderer(templateRenderer))
	g.Expect(err).Should(Succeed())

	controller := renderer.NewController(engine, valuesForInstance)
	request := &pipeline.Request{
		Instance:  consumerInstance("component"),
		Resources: resources.New(resources.List{*configMap("existing")}),
	}

	err = controller.RenderResources(t.Context(), request)

	g.Expect(err).Should(MatchError(ContainSubstring("render resources")))
	g.Expect(request.Resources.Len()).Should(Equal(1))
	g.Expect(request.Resources.Get()[0].GetName()).Should(Equal("existing"))
}

func newEngine(t *testing.T) (*manifestengine.Engine, string) {
	t.Helper()
	g := NewWithT(t)

	chartPath := filepath.Join(t.TempDir(), "chart")
	writeFile(t, filepath.Join(chartPath, "Chart.yaml"), helmChartYAML)
	writeFile(t, filepath.Join(chartPath, "templates", "configmap.yaml"), helmTemplateYAML)

	kustomizePath := filepath.Join(t.TempDir(), "kustomize")
	writeFile(t, filepath.Join(kustomizePath, "kustomization.yaml"), kustomizationYAML)
	writeFile(t, filepath.Join(kustomizePath, "configmap.yaml"), kustomizeResourceYAML)

	templateFS := testingfs.MapFS{
		"templates/configmap.yaml.tmpl": &testingfs.MapFile{
			Data: []byte(templateYAML),
		},
	}

	helmRenderer, err := helm.New(
		[]helm.Source{{Chart: chartPath, ReleaseName: "fixture"}},
		helm.WithCache(),
		helm.WithSourceAnnotations(true),
	)
	g.Expect(err).Should(Succeed())

	kustomizeRenderer, err := kustomize.New(
		[]kustomize.Source{{Path: kustomizePath}},
		kustomize.WithCache(),
		kustomize.WithSourceAnnotations(true),
	)
	g.Expect(err).Should(Succeed())

	templateRenderer, err := gotemplate.New(
		[]gotemplate.Source{{FS: templateFS, Path: "templates/*.yaml.tmpl"}},
		gotemplate.WithCache(),
		gotemplate.WithSourceAnnotations(true),
	)
	g.Expect(err).Should(Succeed())

	engine, err := manifestengine.New(
		manifestengine.WithRenderer(helmRenderer),
		manifestengine.WithRenderer(kustomizeRenderer),
		manifestengine.WithRenderer(templateRenderer),
	)
	g.Expect(err).Should(Succeed())

	return engine, kustomizePath
}

func valuesForInstance(instance api.PlatformObject) map[string]any {
	if instance == nil {
		return nil
	}

	return map[string]any{"name": instance.GetName()}
}

func consumerInstance(name string) *consumer.Consumer {
	return &consumer.Consumer{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func resourceNames(objects resources.List) []string {
	names := make([]string, len(objects))
	for index := range objects {
		names[index] = objects[index].GetName()
	}

	return names
}

func configMap(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name": name,
		},
	}}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	g := NewWithT(t)

	err := os.MkdirAll(filepath.Dir(path), 0o750)
	g.Expect(err).Should(Succeed())

	err = os.WriteFile(path, []byte(content), 0o600)
	g.Expect(err).Should(Succeed())
}
