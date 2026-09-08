//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/onsi/gomega"
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/api/v1alpha1"
)

func chartPath(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}

	return filepath.Clean(filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"config",
		"chart",
	))
}

func installCRD(t *testing.T, kubeClient client.Client) {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve integration test path")
	}

	crdFile := filepath.Clean(filepath.Join(
		filepath.Dir(filename),
		"..",
		"..",
		"config",
		"crd",
		"bases",
		"examples.odh.io_helmcomponents.yaml",
	))

	data, err := os.ReadFile(crdFile)
	g := gomega.NewWithT(t)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	crd := new(apiextensionsv1.CustomResourceDefinition)
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	err = decoder.Decode(crd)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	err = kubeClient.Create(t.Context(), crd)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	g.Eventually(func(g gomega.Gomega) {
		current := new(apiextensionsv1.CustomResourceDefinition)
		err = kubeClient.Get(t.Context(), client.ObjectKeyFromObject(crd), current)
		g.Expect(err).To(gomega.Succeed())
		g.Expect(current.Status.Conditions).To(gomega.ContainElement(gomega.Satisfy(
			func(condition apiextensionsv1.CustomResourceDefinitionCondition) bool {
				return condition.Type == apiextensionsv1.Established &&
					condition.Status == apiextensionsv1.ConditionTrue
			},
		)))
	}).WithContext(t.Context()).WithTimeout(2 * time.Minute).WithPolling(500 * time.Millisecond).Should(gomega.Succeed())
}

func startManager(t *testing.T, manager ctrlmanager.Manager) {
	t.Helper()

	g := gomega.NewWithT(t)
	managerContext, cancelManager := context.WithCancel(t.Context())
	managerDone := make(chan error, 1)
	go func() {
		managerDone <- manager.Start(managerContext)
	}()

	managerFinished := false
	t.Cleanup(func() {
		cancelManager()
		if !managerFinished {
			g.Expect(<-managerDone).To(gomega.Succeed())
		}
	})

	startupContext, cancelStartup := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancelStartup()

	cacheReady := make(chan bool, 1)
	go func() {
		cacheReady <- manager.GetCache().WaitForCacheSync(startupContext)
	}()

	select {
	case err := <-managerDone:
		managerFinished = true
		g.Expect(err).NotTo(gomega.HaveOccurred())
	case synced := <-cacheReady:
		g.Expect(synced).To(gomega.BeTrue())
	case <-startupContext.Done():
		g.Expect(startupContext.Err()).NotTo(gomega.HaveOccurred())
	}
}

func waitForReady(
	t *testing.T,
	kubeClient client.Client,
	component *v1alpha1.HelmComponent,
) {
	t.Helper()

	g := gomega.NewWithT(t)
	g.Eventually(func(g gomega.Gomega) {
		current := v1alpha1.NewHelmComponent()
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(component), current)
		g.Expect(err).To(gomega.Succeed())
		g.Expect(current.Status.Conditions).To(gomega.ContainElement(gomega.Satisfy(
			func(condition platformapi.Condition) bool {
				return condition.Type == string(platformapi.ConditionTypeReady) &&
					condition.Status == metav1.ConditionTrue
			},
		)))
	}).WithContext(t.Context()).WithTimeout(2 * time.Minute).Should(gomega.Succeed())
}

func waitForRenderedConfigMap(
	t *testing.T,
	kubeClient client.Client,
	namespace string,
) {
	t.Helper()

	g := gomega.NewWithT(t)
	expected := &corev1.ConfigMap{}
	expected.Name = "default-helmcomponent-template"
	expected.Namespace = namespace
	g.Eventually(func(g gomega.Gomega) {
		current := expected.DeepCopy()
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(expected), current)
		g.Expect(err).To(gomega.Succeed())
		g.Expect(current.Data).To(gomega.HaveKeyWithValue("source", "helm-controller-example"))
	}).WithContext(t.Context()).WithTimeout(2 * time.Minute).Should(gomega.Succeed())
}

func runtimeUnavailable(err error) bool {
	message := strings.ToLower(err.Error())

	return strings.Contains(message, "failed to get docker info") ||
		strings.Contains(message, "cannot connect to the docker daemon") ||
		strings.Contains(message, "failed to get podman info")
}
