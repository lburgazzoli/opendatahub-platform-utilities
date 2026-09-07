package deploy_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

func TestRunUsesObservabilityCustomizerByDefault(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()

	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	scheme.AddKnownTypeWithName(gvk.MonitoringStack, &unstructured.Unstructured{})

	existing := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.MonitoringStack.GroupVersion().String(),
		"kind":       gvk.MonitoringStack.Kind,
		"metadata": map[string]any{
			"name":      "stack",
			"namespace": "ns",
		},
		"spec": map[string]any{
			"resources": map[string]any{"requests": map[string]any{"cpu": "1"}},
		},
	}}
	existing.SetGroupVersionKind(gvk.MonitoringStack)

	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(existing).Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	desired := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.MonitoringStack.GroupVersion().String(),
		"kind":       gvk.MonitoringStack.Kind,
		"metadata": map[string]any{
			"name":      "stack",
			"namespace": "ns",
		},
		"spec": map[string]any{
			"resources": map[string]any{"requests": map[string]any{"cpu": "2"}},
		},
	}}
	desired.SetGroupVersionKind(gvk.MonitoringStack)

	result, err := deploy.New().Run(t.Context(), deploy.RunOptions{
		Client:    kubernetesClient,
		Owner:     owner,
		Resources: resources.New(resources.List{desired}),
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.Applied).Should(Equal(1))

	stored := &unstructured.Unstructured{}
	stored.SetGroupVersionKind(gvk.MonitoringStack)

	g.Expect(kubernetesClient.Get(
		t.Context(),
		client.ObjectKey{Namespace: "ns", Name: "stack"},
		stored,
	)).Should(Succeed())
	resourcesValue, found, err := unstructured.NestedString(
		stored.Object,
		"spec",
		"resources",
		"requests",
		"cpu",
	)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(found).Should(BeTrue())
	g.Expect(resourcesValue).Should(Equal("1"))
}
