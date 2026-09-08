//go:build integration

package integration_test

import (
	"testing"

	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	kindtest "github.com/opendatahub-io/odh-platform-utilities/testkit/kind"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/api/v1alpha1"
	moduleconfig "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/config"
	modulemanager "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/manager"
)

func TestHelmBuilderReconcilesOnKind(t *testing.T) {
	t.Parallel()

	g := gomega.NewWithT(t)

	engine := kindtest.New()
	cluster, err := engine.Start(t.Context())
	if err != nil {
		if runtimeUnavailable(err) {
			t.Skipf("container runtime is unavailable: %v", err)
		}

		g.Expect(err).NotTo(gomega.HaveOccurred())
	}

	t.Cleanup(func() {
		g.Expect(engine.Close(t.Context())).To(gomega.Succeed())
	})

	scheme := runtime.NewScheme()
	g.Expect(v1alpha1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(apiextensionsv1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())

	kubeClient, err := cluster.Client(scheme)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	installCRD(t, kubeClient)

	configuration := &moduleconfig.Config{
		ChartPath:              chartPath(t),
		Namespace:              "default",
		HealthProbeBindAddress: "0",
	}
	manager, err := modulemanager.New(cluster.RESTConfig(), configuration)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	startManager(t, manager)

	component := v1alpha1.NewHelmComponent()
	component.Name = "default-helmcomponent"
	g.Expect(kubeClient.Create(t.Context(), component)).To(gomega.Succeed())

	waitForReady(t, kubeClient, component)
	waitForRenderedConfigMap(t, kubeClient, configuration.Namespace)
}
