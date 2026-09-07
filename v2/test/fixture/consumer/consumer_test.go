package consumer

import (
	"testing"

	. "github.com/onsi/gomega"
	api "github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

var (
	_ api.PlatformObject          = (*Consumer)(nil)
	_ api.PlatformObject          = (*OptionalConsumer)(nil)
	_ api.ConditionsAccessor      = (*OptionalConsumer)(nil)
	_ api.PhaseStatusAccessor     = (*OptionalConsumer)(nil)
	_ api.PlatformProfileAccessor = (*OptionalConsumer)(nil)
)

func TestConsumerContractAndDeepCopy(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	consumer := &Consumer{}
	consumer.Status.ObservedGeneration = 4
	consumer.Status.Releases = []api.ComponentRelease{{Name: "platform", Version: "2.0.0"}}

	cloned := consumer.DeepCopy()
	cloned.Status.Releases[0].Version = "changed"

	g.Expect(consumer.GetStatus().ObservedGeneration).Should(Equal(int64(4)))
	g.Expect(consumer.Status.ReleaseStatus.Releases[0].Version).Should(Equal("2.0.0"))
	g.Expect(cloned).ShouldNot(BeIdenticalTo(consumer))
}

func TestOptionalConsumerCapabilities(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	consumer := &OptionalConsumer{}
	profile := api.PlatformProfile{Kind: "ODH", Annotations: map[string]string{"example.io/fact": "value"}}
	condition := api.Condition{Type: string(api.ConditionTypeReady)}

	consumer.SetConditions([]api.Condition{condition})
	consumer.SetPhaseStatus(api.PhaseStatus{Phase: api.PhaseReady})
	consumer.SetPlatformProfile(profile)

	g.Expect(consumer.GetConditions()).Should(HaveLen(1))
	g.Expect(consumer.GetPhaseStatus().Phase).Should(Equal(api.PhaseReady))
	g.Expect(consumer.GetPlatformProfile()).Should(Equal(&profile))
}
