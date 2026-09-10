package gc_test

import (
	"context"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/gc"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/ownership"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	platformmetadata "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata"
	"github.com/opendatahub-io/odh-platform-utilities/v2/test/fixture/consumer"
)

func TestRunDeletesOnlyOwnedStaleResources(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	scheme, restMapper := testScheme()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:       "owner",
		Namespace:  "ns",
		UID:        "owner-uid",
		Generation: 2,
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "ns"}}
	stale := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "stale", Namespace: "ns"}}
	unowned := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "unowned", Namespace: "ns"}}
	optOut := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "opt-out", Namespace: "ns"}}
	policy := platformmetadata.DefaultPolicy()
	for _, object := range []*corev1.ConfigMap{desired, stale, optOut} {
		g.Expect(policy.Apply(object, owner)).Should(Succeed())
		g.Expect(ownership.SetControllerReference(owner, object, scheme)).Should(Succeed())
	}
	stale.Annotations["platform.opendatahub.io/instance.generation"] = "1"
	optOut.Annotations["opendatahub.io/managed"] = "false"

	kubernetesClient := authorizedClient(scheme, restMapper, owner, desired, stale, unowned, optOut)
	collection := resources.New(resourceList(t, scheme, desired))

	result, err := gc.New(gc.StaticDiscovery(corev1.SchemeGroupVersion.WithKind("ConfigMap"))).Run(
		t.Context(),
		gc.RunOptions{Client: kubernetesClient, Owner: owner, Resources: collection},
	)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.Discovered).Should(Equal(1))
	g.Expect(result.Deleted).Should(Equal(1))

	for _, name := range []string{"desired", "unowned", "opt-out"} {
		object := &corev1.ConfigMap{}
		err = kubernetesClient.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: name}, object)
		g.Expect(err).ShouldNot(HaveOccurred())
	}
	err = kubernetesClient.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "stale"}, &corev1.ConfigMap{})
	g.Expect(err).Should(HaveOccurred())
	g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
}

func TestExecuteUsesRunContract(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	scheme, restMapper := testScheme()
	consumerGroupVersion := schema.GroupVersion{Group: "test.opendatahub.io", Version: "v1"}
	scheme.AddKnownTypes(consumerGroupVersion, &consumer.Consumer{}, &consumer.ConsumerList{})
	owner := &consumer.Consumer{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid", Generation: 1,
	}}
	owner.SetGroupVersionKind(consumerGroupVersion.WithKind("Consumer"))
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "ns"}}
	collection := resources.New(resourceList(t, scheme, desired))
	kubernetesClient := authorizedClient(scheme, restMapper, owner)

	err := gc.New(gc.StaticDiscovery(corev1.SchemeGroupVersion.WithKind("ConfigMap"))).Execute(
		t.Context(),
		&pipeline.Request{Client: kubernetesClient, Instance: owner, Resources: collection},
	)
	g.Expect(err).ShouldNot(HaveOccurred())
}

func TestRunRejectsEmptyDiscoveryBeforeIO(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	_, err := gc.New(gc.StaticDiscovery()).Run(t.Context())
	g.Expect(err).Should(MatchError(gc.ErrEmptyDiscovery))

	_, err = gc.New(gc.StaticDiscovery(corev1.SchemeGroupVersion.WithKind("ConfigMap"))).Run(t.Context())
	g.Expect(err).Should(MatchError(gc.ErrRunInputRequired))

	scheme, restMapper := testScheme()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	kubernetesClient := authorizedClient(scheme, restMapper, owner)
	_, err = gc.New(gc.StaticDiscovery()).Run(t.Context(), gc.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: resources.New(nil),
	})
	g.Expect(err).Should(MatchError(gc.ErrEmptyDiscovery))
}

func TestRunValidatesActionBeforeRunInputs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := gc.New(nil)

	_, err := action.Run(t.Context())

	g.Expect(err).Should(MatchError(gc.ErrTypeDiscoveryRequired))
}

func TestRunRejectsUnscopedSelectorBeforeListing(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	scheme, restMapper := testScheme()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	kubernetesClient := authorizedClient(scheme, restMapper, owner)
	_, err := gc.New(
		gc.StaticDiscovery(corev1.SchemeGroupVersion.WithKind("ConfigMap")),
		gc.WithMetadataPolicy(unscopedPolicy{}),
	).Run(t.Context(), gc.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: resources.New(nil),
	})
	g.Expect(err).Should(MatchError(gc.ErrUnscopedSelector))
}

func testScheme() (*runtime.Scheme, *meta.DefaultRESTMapper) {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = authorizationv1.AddToScheme(scheme)
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	return scheme, restMapper
}

func authorizedClient(
	scheme *runtime.Scheme,
	restMapper meta.RESTMapper,
	objects ...client.Object,
) client.Client {
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(restMapper).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			Create: func(
				_ context.Context,
				_ client.WithWatch,
				object client.Object,
				_ ...client.CreateOption,
			) error {
				if review, ok := object.(*authorizationv1.SelfSubjectAccessReview); ok {
					review.Status.Allowed = true
					return nil
				}
				return nil
			},
		}).Build()
}

type unscopedPolicy struct{}

func (unscopedPolicy) Apply(client.Object, client.Object) error { return nil }

func (unscopedPolicy) Selector(client.Object) labels.Selector { return labels.Everything() }

func (unscopedPolicy) Matches(client.Object, client.Object) bool { return true }

func resourceList(
	t *testing.T,
	scheme *runtime.Scheme,
	objects ...client.Object,
) resources.List {
	t.Helper()

	list := make(resources.List, len(objects))
	for index, object := range objects {
		u, err := resources.ToUnstructured(object)
		if err != nil {
			t.Fatal(err)
		}

		gvks, _, err := scheme.ObjectKinds(object)
		if err != nil {
			t.Fatal(err)
		}
		u.SetGroupVersionKind(gvks[0])
		list[index] = *u
	}

	return list
}
