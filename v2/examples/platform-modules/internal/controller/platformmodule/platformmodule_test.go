package platformmodule

import (
	"path/filepath"
	"testing"

	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	helm "github.com/k8s-manifest-kit/renderer-helm/pkg"
	"github.com/onsi/gomega"
	kservev1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/kserve/v1alpha1"
	v1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInventoryTracksRetainedResourcesAndPrunesOrphans(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "opendatahub-kserve-system"}}
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: "old-controller", Namespace: namespace.Name,
	}}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namespace, serviceAccount).Build()

	module := v1alpha1.NewPlatformModule()
	module.Name = "kserve"
	module.Spec.Module = "kserve"
	module.Status.Resources = []v1alpha1.ResourceRef{
		{APIVersion: "v1", Kind: "Namespace", Name: namespace.Name},
		{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "kserves.kserve.example.odh.io"},
		{APIVersion: "v1", Kind: "ServiceAccount", Namespace: namespace.Name, Name: serviceAccount.Name},
	}

	currentDeployment := new(unstructured.Unstructured)
	currentDeployment.SetAPIVersion("apps/v1")
	currentDeployment.SetKind("Deployment")
	currentDeployment.SetNamespace(namespace.Name)
	currentDeployment.SetName("new-controller")

	request := &pipeline.Request{
		Client:    kubeClient,
		Instance:  module,
		Resources: resources.New(resources.List{*currentDeployment, *currentDeployment}),
	}
	controller := new(Controller)

	g.Expect(controller.pruneOrphans(t.Context(), request)).To(gomega.Succeed())
	g.Expect(controller.recordResources(t.Context(), request)).To(gomega.Succeed())

	err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(serviceAccount), new(corev1.ServiceAccount))
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(namespace), new(corev1.Namespace))
	g.Expect(err).To(gomega.Succeed())
	g.Expect(module.Status.Resources).To(gomega.Equal([]v1alpha1.ResourceRef{
		{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "kserves.kserve.example.odh.io"},
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: namespace.Name, Name: "new-controller"},
		{APIVersion: "v1", Kind: "Namespace", Name: namespace.Name},
	}))
}

func TestCleanupWaitsForConfiguredModuleCRAndRetainsNamespaceAndCRD(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	registry, err := modules.Load(filepath.Join("..", "..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())
	definition, found := registry.Get("kserve")
	g.Expect(found).To(gomega.BeTrue())

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(kservev1alpha1.AddToScheme(scheme)).To(gomega.Succeed())

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: moduleNamespace("kserve")}}
	serviceAccount := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Name: "example-kserve", Namespace: namespace.Name,
	}}
	moduleCR := &kservev1alpha1.Kserve{ObjectMeta: metav1.ObjectMeta{Name: definition.Config.Spec.ModuleRef.Name}}
	kubeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(namespace, serviceAccount, moduleCR).Build()

	module := v1alpha1.NewPlatformModule()
	module.Name = "kserve"
	module.Spec.Module = "kserve"
	module.Status.Resources = []v1alpha1.ResourceRef{
		{APIVersion: "v1", Kind: "Namespace", Name: namespace.Name},
		{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: definition.CRDName},
		{APIVersion: "v1", Kind: "ServiceAccount", Namespace: namespace.Name, Name: serviceAccount.Name},
	}

	controller := &Controller{registry: registry, reader: kubeClient}
	request := &pipeline.Request{Client: kubeClient, Instance: module, Resources: resources.New(nil)}

	err = controller.cleanup(t.Context(), request)
	waitingForCR := "wait for module CR removal: waiting for Kserve/" + moduleCR.Name
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring(waitingForCR)))
	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(serviceAccount), new(corev1.ServiceAccount))
	g.Expect(err).To(gomega.Succeed())

	g.Expect(kubeClient.Delete(t.Context(), moduleCR)).To(gomega.Succeed())
	g.Expect(controller.cleanup(t.Context(), request)).To(gomega.Succeed())

	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(serviceAccount), new(corev1.ServiceAccount))
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(namespace), new(corev1.Namespace))
	g.Expect(err).To(gomega.Succeed())
	g.Expect(module.Status.Resources).To(gomega.HaveLen(3))
	g.Expect(module.Status.Resources[1].Name).To(gomega.Equal(definition.CRDName))
}

func TestChartValuesToValues(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	spec := modules.ModuleSpec{
		ModuleRef:     modules.ModuleRef{APIVersion: "example.io/v1", Kind: "Example", Name: "cluster"},
		Chart:         modules.ChartSpec{Name: "example", Path: "charts", Version: "1.0.0"},
		RelatedImages: []string{"example.io/controller:1.0.0"},
		Config:        map[string]any{"replicas": 2},
		Services:      []string{"example"},
		Runlevel:      3,
	}
	configured := ChartValues{
		Module: ModuleValues{
			ModuleSpec: spec,
			Namespace:  "opendatahub-example-system",
			Image:      "example.io/controller:1.0.0",
			Enabled:    true,
		},
		Projections: ProjectionValues{Enabled: false},
	}
	values, err := configured.ToValues()
	g.Expect(err).To(gomega.Succeed())
	g.Expect(values).To(gomega.Equal(manifesttypes.Values{
		"module": map[string]any{
			"moduleRef":     map[string]any{"apiVersion": "example.io/v1", "kind": "Example", "name": "cluster"},
			"chart":         map[string]any{"name": "example", "path": "charts", "version": "1.0.0"},
			"relatedImages": []string{"example.io/controller:1.0.0"},
			"config":        map[string]any{"replicas": 2},
			"services":      []string{"example"},
			"runlevel":      3,
			"namespace":     "opendatahub-example-system",
			"image":         "example.io/controller:1.0.0",
			"enabled":       true,
		},
		"projections": map[string]any{"enabled": false},
	}))

	minimal := ChartValues{Module: ModuleValues{ModuleSpec: modules.ModuleSpec{ModuleRef: spec.ModuleRef}}}
	minimalValues, err := minimal.ToValues()
	g.Expect(err).To(gomega.Succeed())
	g.Expect(minimalValues).To(gomega.Equal(manifesttypes.Values{
		"module": map[string]any{
			"moduleRef": map[string]any{"apiVersion": "example.io/v1", "kind": "Example", "name": "cluster"},
			"namespace": "",
			"image":     "",
			"enabled":   false,
		},
		"projections": map[string]any{"enabled": false},
	}))
}

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
			ReleaseNamespace:    "opendatahub-" + name + "-system",
			ReleaseVersion:      definition.Config.Spec.Chart.Version,
			ProcessDependencies: true,
		})
		g.Expect(err).To(gomega.Succeed())

		spec := definition.Config.Spec
		spec.Config = map[string]any{"replicas": 2}
		configuredValues := ChartValues{
			Module: ModuleValues{
				ModuleSpec: spec,
				Enabled:    true,
				Namespace:  moduleNamespace(name),
				Image:      "ttl.sh/example:24h",
			},
			Projections: ProjectionValues{Enabled: false},
		}
		values, err := configuredValues.ToValues()
		g.Expect(err).To(gomega.Succeed())

		objects, err := renderer.Render(t.Context(), manifestrender.WithValues(values))
		g.Expect(err).To(gomega.Succeed())
		g.Expect(objects).To(gomega.HaveLen(6))

		var deployment *unstructured.Unstructured
		var crd *unstructured.Unstructured
		var namespace *unstructured.Unstructured
		for index := range objects {
			switch objects[index].GetKind() {
			case "Deployment":
				deployment = &objects[index]
			case "CustomResourceDefinition":
				crd = &objects[index]
			case "Namespace":
				namespace = &objects[index]
			case "ServiceAccount":
				g.Expect(objects[index].GetNamespace()).To(gomega.Equal("opendatahub-" + name + "-system"))
			}
		}
		g.Expect(deployment).NotTo(gomega.BeNil())
		g.Expect(deployment.GetNamespace()).To(gomega.Equal("opendatahub-" + name + "-system"))
		g.Expect(namespace).NotTo(gomega.BeNil())
		g.Expect(namespace.GetName()).To(gomega.Equal("opendatahub-" + name + "-system"))
		assertModuleCRD(t, crd, definition)

		assertModuleDeployment(t, deployment, name)
	}
}

func assertModuleDeployment(t *testing.T, deployment *unstructured.Unstructured, name string) {
	t.Helper()
	g := gomega.NewWithT(t)

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
