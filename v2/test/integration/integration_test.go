//go:build integration

package integration_test

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kindtest "github.com/opendatahub-io/odh-platform-utilities/testkit/kind"
	v2manager "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/manager"
	platformmetadata "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
)

func TestControllerLifecycleOnKind(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	engine := kindtest.New(kindtest.WithKeep(os.Getenv("KEEP_KIND_CLUSTER") == "true"))
	cluster, err := engine.Start(t.Context())
	if err != nil {
		if runtimeUnavailable(err) {
			t.Skipf("container runtime is unavailable: %v", err)
		}

		g.Expect(err).NotTo(HaveOccurred())
	}

	t.Cleanup(func() {
		closeErr := engine.Close(context.WithoutCancel(t.Context()))
		g.Expect(closeErr).To(Succeed())
	})

	scheme := integrationScheme(t)
	kubeClient, err := cluster.Client(scheme)
	g.Expect(err).NotTo(HaveOccurred())

	installComponentCRD(t, kubeClient)
	component := createComponent(t, kubeClient)
	createStaleConfigMap(t, kubeClient, scheme, component)

	runtimeManager, err := newRuntimeManager(cluster.RESTConfig(), scheme)
	g.Expect(err).NotTo(HaveOccurred())

	wrappedManager := v2manager.New(runtimeManager)
	g.Expect(setupController(t, wrappedManager)).To(Succeed())
	startManager(t, wrappedManager)

	waitForManagedConfigMap(t, kubeClient, component)
	waitForStaleConfigMapDeletion(t, kubeClient, component.Namespace)
	waitForStatus(t, kubeClient, component)

	managed := new(corev1.ConfigMap)
	g.Expect(kubeClient.Get(t.Context(), client.ObjectKey{
		Namespace: component.Namespace,
		Name:      managedConfigMapName(component),
	}, managed)).To(Succeed())
	g.Expect(managed.OwnerReferences).To(HaveLen(1))
	g.Expect(managed.OwnerReferences[0].UID).To(Equal(component.UID))
	g.Expect(managed.OwnerReferences[0].Controller).NotTo(BeNil())
	g.Expect(*managed.OwnerReferences[0].Controller).To(BeTrue())
	g.Expect(managed.Data).To(HaveKeyWithValue("source", "kind-integration"))

	g.Expect(kubeClient.Delete(t.Context(), component)).To(Succeed())
	waitForDeletion(t, kubeClient, component, managed)
}

func createComponent(t *testing.T, kubeClient client.Client) *consumer.Consumer {
	t.Helper()

	component := &consumer.Consumer{}
	component.SetGroupVersionKind(componentGVK)
	component.SetNamespace(testNamespace)
	component.SetName("kind-component")

	g := NewWithT(t)
	g.Expect(kubeClient.Create(t.Context(), component)).To(Succeed())

	created := &consumer.Consumer{}
	created.SetGroupVersionKind(componentGVK)
	g.Expect(kubeClient.Get(t.Context(), client.ObjectKeyFromObject(component), created)).To(Succeed())

	return created
}

func createStaleConfigMap(
	t *testing.T,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	component *consumer.Consumer,
) {
	t.Helper()

	stale := staleConfigMap(component.Namespace)
	g := NewWithT(t)
	g.Expect(controllerutil.SetControllerReference(component, stale, scheme)).To(Succeed())
	g.Expect(platformmetadata.DefaultPolicy().Apply(stale, component)).To(Succeed())
	g.Expect(kubeClient.Create(t.Context(), stale)).To(Succeed())
}

func waitForManagedConfigMap(t *testing.T, kubeClient client.Client, component *consumer.Consumer) {
	t.Helper()

	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		managed := new(corev1.ConfigMap)
		err := kubeClient.Get(t.Context(), client.ObjectKey{
			Namespace: component.Namespace,
			Name:      managedConfigMapName(component),
		}, managed)
		g.Expect(err).To(Succeed())
	}).WithContext(t.Context()).WithTimeout(testTimeout).WithPolling(testPolling).Should(Succeed())
}

func waitForStaleConfigMapDeletion(t *testing.T, kubeClient client.Client, namespace string) {
	t.Helper()

	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		stale := new(corev1.ConfigMap)
		err := kubeClient.Get(t.Context(), client.ObjectKey{
			Namespace: namespace,
			Name:      "stale-managed-configmap",
		}, stale)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}).WithContext(t.Context()).WithTimeout(testTimeout).WithPolling(testPolling).Should(Succeed())
}

func waitForStatus(t *testing.T, kubeClient client.Client, component *consumer.Consumer) {
	t.Helper()

	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		current := &consumer.Consumer{}
		current.SetGroupVersionKind(componentGVK)
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(component), current)
		g.Expect(err).To(Succeed())
		g.Expect(current.Status.ObservedGeneration).To(Equal(current.Generation))
	}).WithContext(t.Context()).WithTimeout(testTimeout).WithPolling(testPolling).Should(Succeed())
}

func waitForDeletion(
	t *testing.T,
	kubeClient client.Client,
	component *consumer.Consumer,
	managed *corev1.ConfigMap,
) {
	t.Helper()

	g := NewWithT(t)
	g.Eventually(func(g Gomega) {
		current := &consumer.Consumer{}
		current.SetGroupVersionKind(componentGVK)
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(component), current)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}).WithContext(t.Context()).WithTimeout(testTimeout).WithPolling(testPolling).Should(Succeed())

	g.Eventually(func(g Gomega) {
		current := new(corev1.ConfigMap)
		err := kubeClient.Get(t.Context(), client.ObjectKeyFromObject(managed), current)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}).WithContext(t.Context()).WithTimeout(testTimeout).WithPolling(testPolling).Should(Succeed())
}
