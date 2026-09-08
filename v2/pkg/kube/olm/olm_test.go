package olm_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/olm"
)

func TestOperatorVersion(t *testing.T) {
	t.Parallel()

	operatorCondition := olmObject(
		gvk.OperatorCondition,
		"rhods-operator.1.2.3",
		"operators",
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(operatorCondition).
		Build()

	version, err := olm.OperatorVersion(t.Context(), reader, "operators", "rhods-operator")

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(version).To(Equal("v1.2.3"))
}

func TestOperatorVersionReportsMissingOperator(t *testing.T) {
	t.Parallel()

	reader := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	_, err := olm.OperatorVersion(t.Context(), reader, "operators", "rhods-operator")

	g := NewWithT(t)
	g.Expect(err).To(MatchError(olm.ErrOperatorNotInstalled))
}

func TestNamespacedOLMResources(t *testing.T) {
	t.Parallel()

	subscription := olmObject(
		gvk.Subscription,
		"example",
		"operators",
	)
	catalogSource := olmObject(
		gvk.CatalogSource,
		"catalog",
		"operators",
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(subscription, catalogSource).
		Build()

	g := NewWithT(t)
	exists, err := olm.SubscriptionExists(t.Context(), reader, "operators", "example")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeTrue())

	loaded, err := olm.GetSubscription(t.Context(), reader, "operators", "example")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(loaded.GetName()).To(Equal("example"))

	exists, err = olm.CatalogSourceExists(t.Context(), reader, "operators", "catalog")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeTrue())

	exists, err = olm.CatalogSourceExists(t.Context(), reader, "operators", "missing")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeFalse())
}

func TestNamespacedOLMResourcesRequireNamespace(t *testing.T) {
	t.Parallel()

	reader := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
	g := NewWithT(t)

	exists, err := olm.CatalogSourceExists(t.Context(), reader, "", "catalog")
	g.Expect(exists).To(BeFalse())
	g.Expect(err).To(MatchError(olm.ErrNamespaceRequired))

	_, err = olm.GetSubscription(t.Context(), reader, "", "subscription")
	g.Expect(err).To(MatchError(olm.ErrNamespaceRequired))

	_, err = olm.OperatorVersion(t.Context(), reader, "", "rhods-operator")
	g.Expect(err).To(MatchError(olm.ErrNamespaceRequired))
}

func TestOperatorVersionIgnoresOtherNamespaces(t *testing.T) {
	t.Parallel()

	operatorCondition := olmObject(
		gvk.OperatorCondition,
		"rhods-operator.1.2.3",
		"other",
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(operatorCondition).
		Build()

	_, err := olm.OperatorVersion(t.Context(), reader, "operators", "rhods-operator")

	g := NewWithT(t)
	g.Expect(err).To(MatchError(olm.ErrOperatorNotInstalled))
}

func olmObject(gvk schema.GroupVersionKind, name string, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}}
}
