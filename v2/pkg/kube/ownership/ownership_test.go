package ownership_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/ownership"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestOwnerReferenceAndControllerChecks(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())

	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "owner", UID: "owner-uid"}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	child := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "child"}}

	g.Expect(ownership.SetControllerReference(owner, child, scheme)).Should(Succeed())
	g.Expect(ownership.ControlledBy(child, owner)).Should(BeTrue())
	g.Expect(ownership.OwnedBy(child, owner)).Should(BeTrue())
	ref, err := ownership.OwnerRefFrom(owner, scheme)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(ref.UID).Should(Equal(owner.GetUID()))

	var _ client.Object = child
}

func TestOwnerReferenceRejectsMissingScheme(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "owner", UID: "owner-uid"}}
	object := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "child"}}

	_, err := ownership.OwnerRefFrom(owner, nil)
	g.Expect(err).Should(MatchError(ownership.ErrOwnerGVK))
	g.Expect(ownership.SetControllerReference(owner, object, nil)).Should(MatchError(ownership.ErrOwnerGVK))
}
