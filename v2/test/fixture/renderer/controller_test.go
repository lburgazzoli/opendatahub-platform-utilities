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
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/renderer"
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

const templateYAML = `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .name }}-template
data:
  source: gotemplate
`

const invalidTemplate = "{{ .missing"

func TestControllerRendersAndPublishesComposedSources(t *testing.T) {
	t.Parallel()

	engine := newEngine(t)
	controller := renderer.NewController(engine, map[string]any{
		"name": "component",
	})
	request := &pipeline.Request{Resources: resources.New(nil)}

	err := controller.RenderResources(t.Context(), request)

	g := NewWithT(t)
	g.Expect(err).Should(Succeed())
	g.Expect(request.Resources.Len()).Should(Equal(3))
	g.Expect(resourceNames(request.Resources.Get())).Should(ConsistOf(
		"component-helm",
		"component-kustomize",
		"component-template",
	))
	for _, object := range request.Resources.Get() {
		g.Expect(object.GetAnnotations()).Should(HaveKey(manifesttypes.AnnotationSourceType))
	}
}

func TestControllerActionRunsThroughPipeline(t *testing.T) {
	t.Parallel()

	controller := renderer.NewController(newEngine(t), map[string]any{"name": "component"})
	request := &pipeline.Request{Resources: resources.New(nil)}
	pipelineValue := pipeline.New().WithAction(controller.Action())

	err := pipelineValue.Run(t.Context(), request)

	g := NewWithT(t)
	g.Expect(err.Err()).Should(Succeed())
	g.Expect(request.Resources.Len()).Should(Equal(3))
}

func TestControllerDoesNotPublishOnRenderFailure(t *testing.T) {
	t.Parallel()

	invalidTemplate := testingfs.MapFS{
		"templates/configmap.yaml.tmpl": &testingfs.MapFile{
			Data: []byte(invalidTemplate),
		},
	}
	templateRenderer, err := gotemplate.New(
		[]gotemplate.Source{{FS: invalidTemplate, Path: "templates/*.yaml.tmpl"}},
		gotemplate.WithCache(),
	)

	g := NewWithT(t)
	g.Expect(err).Should(Succeed())
	engine, err := manifestengine.New(manifestengine.WithRenderer(templateRenderer))
	g.Expect(err).Should(Succeed())

	controller := renderer.NewController(engine, nil)
	request := &pipeline.Request{Resources: resources.New(resources.List{*configMap("existing")})}

	err = controller.RenderResources(t.Context(), request)

	g.Expect(err).Should(MatchError(ContainSubstring("render resources")))
	g.Expect(request.Resources.Len()).Should(Equal(1))
	g.Expect(request.Resources.Get()[0].GetName()).Should(Equal("existing"))
}

func newEngine(t *testing.T) *manifestengine.Engine {
	t.Helper()

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
	if err != nil {
		t.Fatal(err)
	}

	kustomizeRenderer, err := kustomize.New(
		[]kustomize.Source{{Path: kustomizePath}},
		kustomize.WithCache(),
		kustomize.WithSourceAnnotations(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	templateRenderer, err := gotemplate.New(
		[]gotemplate.Source{{FS: templateFS, Path: "templates/*.yaml.tmpl"}},
		gotemplate.WithCache(),
		gotemplate.WithSourceAnnotations(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	engine, err := manifestengine.New(
		manifestengine.WithRenderer(helmRenderer),
		manifestengine.WithRenderer(kustomizeRenderer),
		manifestengine.WithRenderer(templateRenderer),
	)
	if err != nil {
		t.Fatal(err)
	}

	return engine
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

	err := os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
