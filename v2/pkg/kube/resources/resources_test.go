package resources_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	resources "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestCollectionPreservesOrderAndUsesExplicitWriteBack(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	first := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "first"}}
	second := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "second"}}
	collection := resources.New(resources.List{first, second})

	got := collection.Get()
	got[0] = second

	g.Expect(collection.Get()[0].GetName()).Should(Equal("first"))

	replacement := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "replacement"}}
	g.Expect(collection.SetAt(0, replacement)).Should(Succeed())
	g.Expect(collection.Get()[0].GetName()).Should(Equal("replacement"))
	g.Expect(collection.Filter(func(object client.Object) bool { return object.GetName() != "second" })).Should(Equal(1))
	g.Expect(collection.Get()).Should(HaveLen(1))

	seen := make([]string, 0, 2)
	for _, object := range collection.All() {
		seen = append(seen, object.GetName())
	}

	g.Expect(seen).Should(ConsistOf("replacement"))
}

func TestCollectionTransformIsAtomic(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	collection := resources.New(resources.List{
		&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "first"}},
		&metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "second"}},
	})
	failure := resources.ErrNilTransform

	err := collection.Transform(func(index int, object client.Object) (client.Object, error) {
		if index == 1 {
			return nil, failure
		}

		return &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "changed"}}, nil
	})
	g.Expect(err).Should(MatchError(failure))
	g.Expect(collection.Get()[0].GetName()).Should(Equal("first"))
}

func TestMetadataHelpersSupportPresenceAndValueChecks(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	object := &metav1.PartialObjectMetadata{}
	resources.SetAnnotations(object, map[string]string{"example.io/key": "value"})

	g.Expect(resources.HasAnnotation(object, "example.io/key")).Should(BeTrue())
	g.Expect(resources.HasAnnotation(object, "example.io/key", "value")).Should(BeTrue())
	g.Expect(resources.HasAnnotation(object, "example.io/key", "other")).Should(BeFalse())
	g.Expect(resources.GetAnnotation(object, "example.io/key")).Should(Equal("value"))
}

func TestIdentityAndDecode(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	object := &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{Name: "object"}}
	object.SetGroupVersionKind(schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Object"})
	identity, err := resources.IdentityOf(object, scheme)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(identity.Name).Should(Equal("object"))
	g.Expect(identity.GVK.Kind).Should(Equal("Object"))

	decoded, err := resources.Decode([]byte(
		"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: one\n---\nkind: Service\nmetadata:\n  name: two\n",
	))
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(decoded).Should(HaveLen(2))
	g.Expect(decoded[0].GetName()).Should(Equal("one"))
}

func TestApplyCreatesObjectAndApplyStatusWritesStatus(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	g.Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&appsv1.Deployment{}).
		Build()

	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:            "config",
		ResourceVersion: "server-owned",
		ManagedFields:   []metav1.ManagedFieldsEntry{{Manager: "server"}},
	}, Data: map[string]string{"key": "value"}}
	g.Expect(resources.Apply(t.Context(), kubernetesClient, configMap, client.FieldOwner("test"))).Should(Succeed())

	storedConfigMap := &corev1.ConfigMap{}
	g.Expect(kubernetesClient.Get(t.Context(), client.ObjectKey{Name: "config"}, storedConfigMap)).Should(Succeed())
	g.Expect(storedConfigMap.Data).Should(HaveKeyWithValue("key", "value"))

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "deployment"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}}},
		},
	}
	g.Expect(kubernetesClient.Create(t.Context(), deployment)).Should(Succeed())
	deployment.Status.AvailableReplicas = 1
	g.Expect(resources.ApplyStatus(t.Context(), kubernetesClient, deployment, client.FieldOwner("test"))).Should(Succeed())

	storedDeployment := &appsv1.Deployment{}
	g.Expect(kubernetesClient.Get(t.Context(), client.ObjectKey{Name: "deployment"}, storedDeployment)).Should(Succeed())
	g.Expect(storedDeployment.Status.AvailableReplicas).Should(Equal(int32(1)))
}

func TestApplyStatusReturnsNotFound(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
	baseClient := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appsv1.Deployment{}).Build()
	kubernetesClient := interceptor.NewClient(baseClient, interceptor.Funcs{
		SubResourceApply: func(
			_ context.Context,
			_ client.Client,
			_ string,
			_ runtime.ApplyConfiguration,
			_ ...client.SubResourceApplyOption,
		) error {
			return apierrors.NewNotFound(schema.GroupResource{Group: "apps", Resource: "deployments"}, "missing")
		},
	})
	deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "missing"}}

	g.Expect(resources.ApplyStatus(t.Context(), kubernetesClient, deployment, client.FieldOwner("test"))).Should(
		MatchError(ContainSubstring("not found")),
	)
}
