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

func TestMergeDeploymentsLeavesProbesAsDesired(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	probe := func(path string) map[string]any {
		return map[string]any{"httpGet": map[string]any{"path": path}}
	}
	deployment := func(container map[string]any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{"containers": []any{container}},
				},
			},
		}}
	}

	existing := deployment(map[string]any{
		"name":           "app",
		"livenessProbe":  probe("/live"),
		"readinessProbe": probe("/old-ready"),
		"startupProbe":   probe("/startup"),
	})
	desired := deployment(map[string]any{
		"name":           "app",
		"readinessProbe": probe("/ready"),
	})

	g.Expect(deploy.MergeDeployments(existing, desired)).Should(Succeed())

	containers, found, err := unstructured.NestedSlice(desired.Object, "spec", "template", "spec", "containers")
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(found).Should(BeTrue())
	g.Expect(containers).Should(HaveLen(1))

	container, ok := containers[0].(map[string]any)
	g.Expect(ok).Should(BeTrue())
	g.Expect(container).ShouldNot(HaveKey("livenessProbe"))
	g.Expect(container).ShouldNot(HaveKey("startupProbe"))

	path, found, err := unstructured.NestedString(container, "readinessProbe", "httpGet", "path")
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(found).Should(BeTrue())
	g.Expect(path).Should(Equal("/ready"))
}

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
		Resources: resources.New(resourceList(t, scheme, desired)),
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
