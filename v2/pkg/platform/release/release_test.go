package release_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/release"
)

type accessor struct{ status api.ReleaseStatus }

func (a *accessor) GetReleaseStatus() *api.ReleaseStatus     { return &a.status }
func (a *accessor) SetReleaseStatus(value api.ReleaseStatus) { a.status = value }

func TestReleaseOperationsUpsertAndCopy(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	status := &accessor{}
	g.Expect(release.Set(status, api.ComponentRelease{Name: "component", Version: "v1"})).Should(BeTrue())
	g.Expect(release.SetPlatformVersion(status, "v2")).Should(BeTrue())
	g.Expect(release.PlatformVersion(status)).Should(Equal("v2"))

	value := release.Get(status, "component")
	g.Expect(value).ShouldNot(BeNil())
	value.Version = "mutated"
	g.Expect(release.Get(status, "component").Version).Should(Equal("v1"))
	g.Expect(release.Set(status, api.ComponentRelease{Name: "component", Version: "v1"})).Should(BeFalse())
}
