package deploy_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

func TestRunCoreOptionsCompose(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())

	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	owner := &corev1.ConfigMap{}
	collection := resources.New(nil)
	var configured deploy.RunOptions

	g.Expect(configured.Validate()).Should(MatchError(ContainSubstring("deploy client")))
	configured.Merge(
		deploy.WithRunClient(kubernetesClient),
		deploy.WithRunOwner(owner),
		deploy.WithRunResources(collection),
		deploy.WithRunFieldOwner("run-owner"),
	)
	g.Expect(configured.Validate()).Should(Succeed())

	g.Expect(configured.Client).Should(BeIdenticalTo(kubernetesClient))
	g.Expect(configured.Owner).Should(BeIdenticalTo(owner))
	g.Expect(configured.Resources).Should(BeIdenticalTo(collection))
	g.Expect(configured.FieldOwner).Should(Equal("run-owner"))

	configured.Merge(deploy.RunOptions{FieldOwner: "struct-owner"})
	g.Expect(configured.FieldOwner).Should(Equal("struct-owner"))
}

func TestRunMetadataOptionsCopyAndCompose(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	var unset deploy.RunOptions
	unset.Merge(deploy.WithRunLabels(nil), deploy.WithRunAnnotations(nil))
	g.Expect(unset.Labels).Should(BeNil())
	g.Expect(unset.Annotations).Should(BeNil())

	labels := map[string]string{"example.io/first": "original"}
	annotations := map[string]string{"example.io/first": "original"}
	var configured deploy.RunOptions

	configured.Merge(deploy.RunOptions{Labels: labels, Annotations: annotations})
	labels["example.io/first"] = "changed"
	annotations["example.io/first"] = "changed"

	g.Expect(configured.Labels).Should(HaveKeyWithValue("example.io/first", "original"))
	g.Expect(configured.Annotations).Should(HaveKeyWithValue("example.io/first", "original"))

	configured.Merge(
		deploy.WithRunLabels(map[string]string{"example.io/second": "second"}),
		deploy.WithRunAnnotations(map[string]string{"example.io/second": "second"}),
		deploy.WithRunLabel("example.io/first", "replacement"),
		deploy.WithRunAnnotation("example.io/first", "replacement"),
	)

	g.Expect(configured.Labels).Should(Equal(map[string]string{
		"example.io/first":  "replacement",
		"example.io/second": "second",
	}))
	g.Expect(configured.Annotations).Should(Equal(map[string]string{
		"example.io/first":  "replacement",
		"example.io/second": "second",
	}))

	configured.Merge(deploy.RunOptions{
		Labels:      map[string]string{},
		Annotations: map[string]string{},
	})
	g.Expect(configured.Labels).Should(BeEmpty())
	g.Expect(configured.Annotations).Should(BeEmpty())

	configured = deploy.RunOptions{Labels: labels, Annotations: annotations}
	configured.Merge(
		deploy.WithRunLabel("example.io/second", "second"),
		deploy.WithRunAnnotation("example.io/second", "second"),
	)
	g.Expect(labels).ShouldNot(HaveKey("example.io/second"))
	g.Expect(annotations).ShouldNot(HaveKey("example.io/second"))
}
