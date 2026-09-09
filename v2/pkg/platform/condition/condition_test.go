package condition_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

type accessor struct{ conditions []api.Condition }

func (a *accessor) GetConditions() []api.Condition       { return a.conditions }
func (a *accessor) SetConditions(values []api.Condition) { a.conditions = values }

func TestConditionMutationAndAggregation(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	conditions := &accessor{}
	g.Expect(condition.MarkFalse(
		conditions,
		"Dependencies",
		condition.WithReason("Unavailable"),
		condition.WithMessagef("missing %s", "API"),
		condition.WithObservedGeneration(0),
	)).Should(BeTrue())
	g.Expect(condition.Find(conditions, "Dependencies").Message).Should(Equal("missing API"))
	g.Expect(condition.Aggregate(conditions, "Ready", "Dependencies")).Should(BeTrue())
	g.Expect(condition.IsFalse(conditions, "Ready")).Should(BeTrue())

	g.Expect(condition.MarkTrue(
		conditions,
		"Dependencies",
		condition.WithSeverity(api.ConditionSeverityInfo),
	)).Should(BeTrue())
	g.Expect(condition.Aggregate(conditions, "Ready", "Dependencies")).Should(BeTrue())
	g.Expect(condition.IsTrue(conditions, "Ready")).Should(BeTrue())

	g.Expect(condition.MarkUnknown(
		conditions,
		"Failure",
		condition.WithError(condition.ErrNilAccessor),
	)).Should(BeTrue())
	g.Expect(condition.Find(conditions, "Failure").Reason).Should(Equal("Error"))
}

func TestConditionMutationPreservesTransitionTimeAndCopiesFindResult(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	conditions := &accessor{}
	g.Expect(condition.MarkTrue(conditions, "Ready")).Should(BeTrue())
	first := condition.Find(conditions, "Ready")
	g.Expect(first).ShouldNot(BeNil())
	g.Expect(condition.MarkTrue(conditions, "Ready", condition.WithReason("Healthy"))).Should(BeTrue())
	second := condition.Find(conditions, "Ready")
	g.Expect(second.LastTransitionTime).Should(Equal(first.LastTransitionTime))
	second.Reason = "mutated"
	g.Expect(condition.Find(conditions, "Ready").Reason).Should(Equal("Healthy"))
	g.Expect(condition.Remove(conditions, "Ready")).Should(BeTrue())
	g.Expect(condition.Find(conditions, "Ready")).Should(BeNil())

	var _ condition.MarkOption = condition.MarkOptions{}
}
