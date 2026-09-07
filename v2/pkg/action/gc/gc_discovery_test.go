package gc_test

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/gc"
)

func TestDynamicDiscoveryIsUncachedWithoutEventInvalidation(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	resourceLists := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Verbs: []string{"get", "list"}},
				{Name: "configmaps/status", Kind: "ConfigMap", Verbs: []string{"get", "list"}},
				{Name: "secrets", Kind: "Secret", Verbs: []string{"get"}},
			},
		},
		{
			GroupVersion: "apps/v1",
			APIResources: []metav1.APIResource{{Name: "deployments", Kind: "Deployment", Verbs: []string{"list"}}},
		},
	}
	discoveryClient := &discoveryMock{}
	discoveryClient.On("ServerPreferredResources").Return(resourceLists, nil).Twice()
	discoverer := gc.DynamicDiscovery(discoveryClient)

	first, err := discoverer.Discover(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	second, err := discoverer.Discover(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(first).Should(ConsistOf(
		schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"},
		schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
	))
	g.Expect(second).Should(Equal(first))
	discoveryClient.AssertExpectations(t)
}

func TestStaticDiscoveryCopiesInput(t *testing.T) {
	t.Parallel()
	g := NewWithT(t)

	types := []schema.GroupVersionKind{{Version: "v1", Kind: "ConfigMap"}}
	discoverer := gc.StaticDiscovery(types...)
	types[0] = schema.GroupVersionKind{Version: "v1", Kind: "Secret"}

	result, err := discoverer.Discover(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result).Should(Equal([]schema.GroupVersionKind{{Version: "v1", Kind: "ConfigMap"}}))
}

//nolint:govet // embedded mocks mirror the discovery interface without reordering fields.
type discoveryMock struct {
	mock.Mock
	discovery.DiscoveryInterface
}

func (d *discoveryMock) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	arguments := d.Called()
	resources, _ := arguments.Get(0).([]*metav1.APIResourceList)
	return resources, arguments.Error(1)
}

var _ discovery.DiscoveryInterface = (*discoveryMock)(nil)
