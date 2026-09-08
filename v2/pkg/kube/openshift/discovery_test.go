package openshift_test

import (
	"testing"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/openshift"
)

func TestOpenShiftDiscovery(t *testing.T) {
	t.Parallel()

	clusterVersion := discoveryObject(
		gvk.ClusterVersion,
		"",
		"version",
		map[string]any{"status": map[string]any{
			"history": []any{map[string]any{"version": "4.18.2"}},
		}},
	)
	authentication := discoveryObject(
		gvk.Authentication,
		"",
		"cluster",
		map[string]any{"spec": map[string]any{
			"type":                 openshift.AuthenticationOIDC,
			"serviceAccountIssuer": "https://issuer.example",
		}},
	)
	ingress := discoveryObject(
		gvk.Ingress,
		"",
		"cluster",
		map[string]any{"spec": map[string]any{
			"appsDomain": "apps.example",
		}},
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(clusterVersion, authentication, ingress).
		Build()

	version, err := openshift.GetVersion(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(version).To(Equal("4.18.2"))

	mode, err := openshift.GetAuthenticationMode(t.Context(), reader)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(mode).To(Equal(openshift.AuthenticationOIDC))

	issuer, err := openshift.GetServiceAccountIssuer(t.Context(), reader)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(issuer).To(Equal("https://issuer.example"))

	domain, err := openshift.GetDomain(t.Context(), reader)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(domain).To(Equal("apps.example"))
}

func TestIsSingleNodeClusterUsesInfrastructure(t *testing.T) {
	t.Parallel()

	infrastructure := discoveryObject(
		gvk.Infrastructure,
		"",
		"cluster",
		map[string]any{"status": map[string]any{
			"controlPlaneTopology": "SingleReplica",
		}},
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(infrastructure).
		Build()

	singleNode, err := openshift.IsSingleNodeCluster(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(singleNode).To(BeTrue())
}

func TestIsSingleNodeClusterFallsBackToNodes(t *testing.T) {
	t.Parallel()

	node := discoveryObject(
		gvk.Node,
		"",
		"node-1",
		map[string]any{"spec": map[string]any{"unschedulable": false}},
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(node).
		Build()

	singleNode, err := openshift.IsSingleNodeCluster(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(singleNode).To(BeTrue())
}

func TestIsFIPSEnabled(t *testing.T) {
	t.Parallel()

	configMap := discoveryObject(
		gvk.ConfigMap,
		"kube-system",
		"cluster-config-v1",
		map[string]any{"data": map[string]any{
			"install-config": "fips: true\n",
		}},
	)
	reader := fake.NewClientBuilder().
		WithScheme(runtime.NewScheme()).
		WithObjects(configMap).
		Build()

	fipsEnabled, err := openshift.IsFIPSEnabled(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(fipsEnabled).To(BeTrue())
}

func discoveryObject(
	gvk schema.GroupVersionKind,
	namespace string,
	name string,
	fields map[string]any,
) *unstructured.Unstructured {
	value := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": gvk.GroupVersion().String(),
		"kind":       gvk.Kind,
		"metadata": map[string]any{
			"name":      name,
			"namespace": namespace,
		},
	}}

	for key, field := range fields {
		value.Object[key] = field
	}

	return value
}
