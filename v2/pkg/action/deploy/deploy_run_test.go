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
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
)

func TestRunNormalizesPublishesAndApplies(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "owner-ns", UID: "owner-uid", Generation: 3,
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "owner-ns"}}
	collection := resources.New(resources.List{desired})

	result, err := deploy.New(
		deploy.WithLabel("example.io/test", "true"),
	).Run(t.Context(), deploy.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: collection,
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.Applied).Should(Equal(1))
	g.Expect(desired.GetNamespace()).Should(Equal("owner-ns"))
	g.Expect(desired.GetLabels()).Should(BeEmpty())
	g.Expect(desired.GetAnnotations()).Should(BeEmpty())

	prepared := collection.Get()
	g.Expect(prepared).Should(HaveLen(1))
	g.Expect(prepared[0]).ShouldNot(BeIdenticalTo(desired))
	g.Expect(prepared[0].GetLabels()).Should(HaveKeyWithValue("example.io/test", "true"))
	g.Expect(prepared[0].GetLabels()).Should(HaveKeyWithValue(labels.PlatformPartOf, "configmap"))
	g.Expect(prepared[0].GetAnnotations()).Should(HaveKeyWithValue(annotations.InstanceName, "owner"))
	g.Expect(prepared[0].GetAnnotations()).Should(HaveKeyWithValue(annotations.InstanceUID, "owner-uid"))

	stored := &corev1.ConfigMap{}
	stored.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	g.Expect(kubernetesClient.Get(
		t.Context(),
		client.ObjectKey{Namespace: "owner-ns", Name: "desired"},
		stored,
	)).Should(Succeed())
	g.Expect(metav1.IsControlledBy(stored, owner)).Should(BeTrue())
}

func TestRunDoesNotPublishPreparedObjectsAfterValidationFailure(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	first := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "ns"}}
	second := &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{"name": "second", "namespace": "ns"},
	}}
	collection := resources.New(resources.List{first, second})

	_, err := deploy.New(deploy.WithLabel("example.io/test", "true")).Run(t.Context(), deploy.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: collection,
	})
	g.Expect(err).Should(MatchError(ContainSubstring("identify resource")))
	g.Expect(first.GetObjectKind().GroupVersionKind()).Should(BeZero())
	g.Expect(first.GetLabels()).Should(BeEmpty())
	g.Expect(first.GetAnnotations()).Should(BeEmpty())

	published := collection.Get()
	g.Expect(published).Should(HaveLen(2))
	g.Expect(published[0]).Should(BeIdenticalTo(first))
	g.Expect(published[1]).Should(BeIdenticalTo(second))
}
