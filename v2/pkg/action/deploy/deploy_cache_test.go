package deploy_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

func TestRunUsesCacheForIdenticalDesiredResources(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	kubernetesClient := &countingClient{Client: baseClient}
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "ns"}}
	collection := resources.New(resources.List{desired})
	action := deploy.New(deploy.WithCache())

	result, err := action.Run(t.Context(), deploy.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: collection,
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.Applied).Should(Equal(1))

	result, err = action.Run(t.Context(), deploy.RunOptions{
		Client: kubernetesClient, Owner: owner, Resources: collection,
	})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.Applied).Should(Equal(0))
	g.Expect(result.Skipped).Should(Equal(1))
	g.Expect(kubernetesClient.applyCalls).Should(Equal(1))
}
