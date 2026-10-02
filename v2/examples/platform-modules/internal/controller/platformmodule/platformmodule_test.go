package platformmodule

import (
	"path/filepath"
	"testing"

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
		Resources: resources.New(resources.List{*currentDeployment}),
	}
	controller := new(Controller)

	g.Expect(controller.pruneOrphans(t.Context(), request)).To(gomega.Succeed())
	g.Expect(controller.recordResources(t.Context(), request)).To(gomega.Succeed())

	err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(serviceAccount), new(corev1.ServiceAccount))
	g.Expect(apierrors.IsNotFound(err)).To(gomega.BeTrue())
	err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(namespace), new(corev1.Namespace))
	g.Expect(err).To(gomega.Succeed())
	g.Expect(module.Status.Resources).To(gomega.Equal([]v1alpha1.ResourceRef{
		{APIVersion: "apps/v1", Kind: "Deployment", Namespace: namespace.Name, Name: "new-controller"},
		{APIVersion: "v1", Kind: "Namespace", Name: namespace.Name},
		{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition", Name: "kserves.kserve.example.odh.io"},
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
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("waiting for Kserve/" + moduleCR.Name)))
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
