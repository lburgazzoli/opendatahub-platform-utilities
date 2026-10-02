package deploy_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/deploy"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
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
	collection := resources.New(resourceList(t, scheme, desired))
	action := deploy.New()

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
	g.Expect(kubernetesClient.getCalls).Should(Equal(2))
}

func TestRunMetadataOverridesInvalidateDefaultCache(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())

	kubernetesClient := &countingClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: "owner", Namespace: "ns", UID: "owner-uid",
	}}
	owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
	desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "ns"}}
	action := deploy.New(
		deploy.WithLabel("example.io/label", "default"),
		deploy.WithAnnotation("example.io/annotation", "default"),
	)

	run := func(label string, annotation string) deploy.Result {
		result, err := action.Run(t.Context(), deploy.RunOptions{
			Client:      kubernetesClient,
			Owner:       owner,
			Resources:   resources.New(resourceList(t, scheme, desired)),
			Labels:      map[string]string{"example.io/label": label, labels.PlatformPartOf: "ignored"},
			Annotations: map[string]string{"example.io/annotation": annotation, annotations.InstanceName: "ignored"},
		})
		g.Expect(err).ShouldNot(HaveOccurred())

		return result
	}

	g.Expect(run("first", "first").Applied).Should(Equal(1))
	g.Expect(run("first", "first").Skipped).Should(Equal(1))
	g.Expect(run("second", "first").Applied).Should(Equal(1))
	g.Expect(run("second", "second").Applied).Should(Equal(1))
	g.Expect(kubernetesClient.applyCalls).Should(Equal(3))

	stored := &corev1.ConfigMap{}
	g.Expect(kubernetesClient.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "desired"}, stored)).Should(Succeed())
	g.Expect(stored.GetLabels()).Should(HaveKeyWithValue("example.io/label", "second"))
	g.Expect(stored.GetAnnotations()).Should(HaveKeyWithValue("example.io/annotation", "second"))
	g.Expect(stored.GetLabels()).Should(HaveKeyWithValue(labels.PlatformPartOf, "configmap"))
	g.Expect(stored.GetAnnotations()).Should(HaveKeyWithValue(annotations.InstanceName, "owner"))
}

func TestRunCanDisableCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		option deploy.Option
	}{
		{name: "functional option", option: deploy.WithCache(false)},
		{name: "complete option", option: deploy.Options{Cache: &deploy.CacheOptions{Disabled: true}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			scheme := runtime.NewScheme()
			g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())

			kubernetesClient := &countingClient{Client: fake.NewClientBuilder().WithScheme(scheme).Build()}
			owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
				Name: "owner", Namespace: "ns", UID: "owner-uid",
			}}
			owner.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ConfigMap"))
			desired := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "desired", Namespace: "ns"}}
			action := deploy.New(tt.option)

			for range 2 {
				result, err := action.Run(t.Context(), deploy.RunOptions{
					Client:    kubernetesClient,
					Owner:     owner,
					Resources: resources.New(resourceList(t, scheme, desired)),
				})
				g.Expect(err).ShouldNot(HaveOccurred())
				g.Expect(result.Applied).Should(Equal(1))
			}

			g.Expect(kubernetesClient.applyCalls).Should(Equal(2))
		})
	}
}
