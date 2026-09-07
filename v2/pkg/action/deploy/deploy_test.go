package deploy_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
)

func TestRunRequiresInputs(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	_, err := deploy.New(deploy.WithMode(deploy.ModePatch)).Run(t.Context())
	g.Expect(err).Should(MatchError(ContainSubstring("deploy client")))
}

func TestRunRejectsInvalidStableOptions(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	action := deploy.New(deploy.Options{Mode: deploy.Mode(99)})

	g.Expect(action.Validate()).Should(MatchError(ContainSubstring("unsupported deploy mode")))
	_, err := action.Run(t.Context())
	g.Expect(err).Should(MatchError(ContainSubstring("unsupported deploy mode")))
}

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
		deploy.Options{Mode: deploy.ModePatch},
		deploy.WithLabel("example.io/test", "true"),
	).Run(t.Context(), deploy.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: collection,
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.Applied).Should(Equal(1))
	g.Expect(desired.GetNamespace()).Should(Equal("owner-ns"))
	g.Expect(desired.GetLabels()).Should(HaveKeyWithValue("example.io/test", "true"))
	g.Expect(desired.GetLabels()).Should(HaveKeyWithValue(labels.PlatformPartOf, "configmap"))
	g.Expect(desired.GetAnnotations()).Should(HaveKeyWithValue(annotations.InstanceName, "owner"))
	g.Expect(desired.GetAnnotations()).Should(HaveKeyWithValue(annotations.InstanceUID, "owner-uid"))

	stored := &corev1.ConfigMap{}
	stored.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	g.Expect(kubernetesClient.Get(
		t.Context(),
		client.ObjectKey{Namespace: "owner-ns", Name: "desired"},
		stored,
	)).Should(Succeed())
	g.Expect(metav1.IsControlledBy(stored, owner)).Should(BeTrue())
}

func TestRunRejectsDuplicateIdentity(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	first := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "same", Namespace: "ns"}}
	second := first.DeepCopy()
	collection := resources.New(resources.List{first, second})

	_, err := deploy.New(deploy.Options{Mode: deploy.ModePatch}).Run(t.Context(), deploy.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: collection,
	})
	g.Expect(err).Should(MatchError(ContainSubstring("duplicate resource identity")))
}

func TestExecuteUsesRunContract(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	consumerGV := schema.GroupVersion{Group: "test.opendatahub.io", Version: "v1"}
	scheme.AddKnownTypes(consumerGV, &consumer.Consumer{}, &consumer.ConsumerList{})
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &consumer.Consumer{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(consumerGV.WithKind("Consumer"))
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "ns"}}
	collection := resources.New(resources.List{desired})

	err := deploy.New(deploy.Options{Mode: deploy.ModePatch}).Execute(t.Context(), &pipeline.Request{
		Client: kubernetesClient, Instance: owner, Resources: collection,
	})
	g.Expect(err).ShouldNot(HaveOccurred())
}
