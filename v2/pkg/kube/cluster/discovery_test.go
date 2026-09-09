package cluster_test

import (
	"testing"

	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/cluster"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
)

func TestDetectClusterDistribution(t *testing.T) {
	t.Parallel()

	t.Run("OpenShift", func(t *testing.T) {
		t.Parallel()

		clusterVersion := object(gvk.ClusterVersion, "", "version", map[string]any{
			"status": map[string]any{
				"history": []any{map[string]any{"version": "4.18.2"}},
			},
		})
		reader := fake.NewClientBuilder().
			WithScheme(runtime.NewScheme()).
			WithObjects(clusterVersion).
			Build()

		distribution, err := cluster.DetectClusterDistribution(t.Context(), reader)

		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(distribution).To(Equal(api.Distribution{
			Kind:    cluster.OpenShift,
			Version: "4.18.2",
		}))
	})

	t.Run("missing API", func(t *testing.T) {
		t.Parallel()

		reader := errorReader{
			Reader: fake.NewClientBuilder().Build(),
			err: &meta.NoKindMatchError{
				GroupKind: schema.GroupKind{Group: "config.openshift.io", Kind: "ClusterVersion"},
			},
		}
		distribution, err := cluster.DetectClusterDistribution(t.Context(), reader)

		g := NewWithT(t)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(distribution).To(Equal(api.Distribution{Kind: cluster.Kubernetes}))
	})

	t.Run("missing singleton is an error", func(t *testing.T) {
		t.Parallel()

		reader := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build()
		_, err := cluster.DetectClusterDistribution(t.Context(), reader)

		g := NewWithT(t)
		g.Expect(err).To(HaveOccurred())
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	})
}

func TestHasCRD(t *testing.T) {
	t.Parallel()

	crd := object(gvk.CustomResourceDefinition, "", "widgets.example.io", map[string]any{
		"status": map[string]any{
			"conditions": []any{map[string]any{
				"type":   "Established",
				"status": "True",
			}},
		},
	})
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(crd).
		Build()

	exists, err := cluster.HasCRD(t.Context(), reader, "widgets.example.io")

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeTrue())

	exists, err = cluster.HasCRD(t.Context(), reader, "missing.example.io")
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(exists).To(BeFalse())
}

func TestHasAPI(t *testing.T) {
	t.Parallel()

	widgetGVK := schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{widgetGVK.GroupVersion()})
	mapper.Add(widgetGVK, meta.RESTScopeNamespace)
	reader := fake.NewClientBuilder().WithRESTMapper(mapper).Build()

	hasAPI, err := cluster.HasAPI(reader, widgetGVK)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(hasAPI).To(BeTrue())

	hasAPI, err = cluster.HasAPI(reader, schema.GroupVersionKind{
		Group:   "missing.example.io",
		Version: "v1",
		Kind:    "Missing",
	})
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(hasAPI).To(BeFalse())
}

func TestHasCRDRejectsMalformedConditions(t *testing.T) {
	t.Parallel()

	crd := object(gvk.CustomResourceDefinition, "", "widgets.example.io", map[string]any{
		"status": map[string]any{
			"conditions": []any{map[string]any{"type": true, "status": "True"}},
		},
	})
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(crd).
		Build()

	_, err := cluster.HasCRD(t.Context(), reader, "widgets.example.io")

	g := NewWithT(t)
	g.Expect(err).To(MatchError(ContainSubstring("type is not a string")))
}
