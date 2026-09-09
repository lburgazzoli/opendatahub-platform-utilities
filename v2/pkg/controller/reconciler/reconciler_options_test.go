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
	conditionTypes := []api.ConditionType{
		api.ConditionTypeDegraded,
		api.ConditionType("DependenciesAvailable"),
		api.ConditionTypeDegraded,
	}
	finalizerName := "example.io/finalizer"

	structOptions := defaultOptions()
	Options{
		CleanupTimeout:   &zero,
		FieldOwner:       "status-owner",
		FinalizerName:    finalizerName,
		PlatformProfile:  &profile,
		DynamicOwnership: true,
		ConditionTypes:   conditionTypes,
	}.ApplyTo(&structOptions)

	functionalOptions := defaultOptions()
	for _, optionValue := range []Option{
		WithCleanupTimeout(0),
		WithFieldOwner("status-owner"),
		WithFinalizerName(finalizerName),
		WithPlatformProfile(profile),
		WithDynamicOwnership(),
		WithConditionTypes(api.ConditionType("DependenciesAvailable"), api.ConditionTypeDegraded),
	} {
		optionValue.ApplyTo(&functionalOptions)
	}
	structOptions.ConditionTypes = normalizeConditionTypes(structOptions.ConditionTypes)
	functionalOptions.ConditionTypes = normalizeConditionTypes(functionalOptions.ConditionTypes)

	g.Expect(structOptions.CleanupTimeout).ShouldNot(BeNil())
	g.Expect(*structOptions.CleanupTimeout).Should(BeZero())
	g.Expect(structOptions).Should(Equal(functionalOptions))
	g.Expect(structOptions.ConditionTypes).Should(Equal([]api.ConditionType{
		api.ConditionTypeDegraded,
		api.ConditionType("DependenciesAvailable"),
		api.ConditionTypeProvisioningSucceeded,
	}))

	profile.Annotations["example.io/key"] = "changed"
	g.Expect(structOptions.PlatformProfile.Annotations["example.io/key"]).Should(Equal("value"))
}
