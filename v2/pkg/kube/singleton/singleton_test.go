package singleton_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/singleton"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGetReturnsTheOnlyObject(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "only"}},
	).Build()
	target := &corev1.ConfigMap{}

	g.Expect(singleton.Get(t.Context(), client, target)).Should(Succeed())
	g.Expect(target.Name).Should(Equal("only"))
}

func TestGetRejectsZeroAndMultipleObjects(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	emptyClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	g.Expect(singleton.Get(t.Context(), emptyClient, &corev1.ConfigMap{})).Should(MatchError(singleton.ErrNoInstance))

	multipleClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "one"}},
		&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "two"}},
	).Build()
	g.Expect(singleton.Get(t.Context(), multipleClient, &corev1.ConfigMap{})).Should(
		MatchError(ContainSubstring(singleton.ErrMultipleInstances.Error())),
	)
}
