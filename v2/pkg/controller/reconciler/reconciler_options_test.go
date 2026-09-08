package reconciler

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

var _ Option = Options{}

func TestOptionsStructAndFunctionalOptionPreserveCleanupPresence(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	zero := time.Duration(0)
	profile := api.PlatformProfile{Kind: "Kubernetes", Annotations: map[string]string{"example.io/key": "value"}}

	structOptions := defaultOptions()
	Options{
		CleanupTimeout:   &zero,
		FieldOwner:       "status-owner",
		PlatformProfile:  &profile,
		DynamicOwnership: true,
	}.ApplyTo(&structOptions)

	functionalOptions := defaultOptions()
	for _, optionValue := range []Option{
		WithCleanupTimeout(0),
		WithFieldOwner("status-owner"),
		WithPlatformProfile(profile),
		WithDynamicOwnership(),
	} {
		optionValue.ApplyTo(&functionalOptions)
	}

	g.Expect(structOptions.CleanupTimeout).ShouldNot(BeNil())
	g.Expect(*structOptions.CleanupTimeout).Should(BeZero())
	g.Expect(structOptions).Should(Equal(functionalOptions))

	profile.Annotations["example.io/key"] = "changed"
	g.Expect(structOptions.PlatformProfile.Annotations["example.io/key"]).Should(Equal("value"))
}
