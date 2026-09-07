package condition

import (
	"errors"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

var ErrNilAccessor = errors.New("condition accessor is required")

// Set upserts a condition while preserving transition time for status-stable
// updates and copying the supplied value into the accessor.
func Set(accessor api.ConditionsAccessor, value api.Condition) bool {
	if accessor == nil {
		return false
	}

	conditions := accessor.GetConditions()

	if value.LastTransitionTime.IsZero() {
		value.LastTransitionTime = metav1.NewTime(time.Now())
	}

	value.LastHeartbeatTime = nil

	for index := range conditions {
		if conditions[index].Type != value.Type {
			continue
		}

		if equal(conditions[index], value) {
			return false
		}

		if conditions[index].Status == value.Status {
			value.LastTransitionTime = conditions[index].LastTransitionTime
		}

		conditions[index] = value
		accessor.SetConditions(slices.Clone(conditions))

		return true
	}

	accessor.SetConditions(append(slices.Clone(conditions), value))

	return true
}

// Mark sets a condition with the supplied status and options.
func Mark(
	accessor api.ConditionsAccessor,
	conditionType string,
	status metav1.ConditionStatus,
	options ...MarkOption,
) bool {
	values := MarkOptions{}

	for _, current := range options {
		if current != nil {
			current.ApplyTo(&values)
		}
	}

	if values.Error != nil {
		values.Severity = api.ConditionSeverityError

		if values.Reason == "" {
			values.Reason = "Error"
		}

		if values.Message == "" {
			values.Message = values.Error.Error()
		}
	}

	return Set(accessor, api.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             values.Reason,
		Message:            values.Message,
		Severity:           values.Severity,
		ObservedGeneration: valueOrZero(values.ObservedGeneration),
	})
}

func MarkTrue(accessor api.ConditionsAccessor, conditionType string, options ...MarkOption) bool {
	return Mark(accessor, conditionType, metav1.ConditionTrue, options...)
}

func MarkFalse(accessor api.ConditionsAccessor, conditionType string, options ...MarkOption) bool {
	return Mark(accessor, conditionType, metav1.ConditionFalse, options...)
}

func MarkUnknown(accessor api.ConditionsAccessor, conditionType string, options ...MarkOption) bool {
	return Mark(accessor, conditionType, metav1.ConditionUnknown, options...)
}

// MarkFrom copies the observable fields from source under a new type.
func MarkFrom(accessor api.ConditionsAccessor, conditionType string, source *api.Condition) bool {
	if source == nil {
		return false
	}

	return Set(accessor, api.Condition{
		Type:     conditionType,
		Status:   source.Status,
		Reason:   source.Reason,
		Message:  source.Message,
		Severity: source.Severity,
	})
}

func Find(accessor api.ConditionsAccessor, conditionType string) *api.Condition {
	if accessor == nil {
		return nil
	}

	for _, value := range accessor.GetConditions() {
		if value.Type == conditionType {
			return value.DeepCopy()
		}
	}

	return nil
}

func Remove(accessor api.ConditionsAccessor, conditionType string) bool {
	if accessor == nil {
		return false
	}

	conditions := accessor.GetConditions()
	for index := range conditions {
		if conditions[index].Type == conditionType {
			updated := append(slices.Clone(conditions[:index]), conditions[index+1:]...)
			accessor.SetConditions(updated)

			return true
		}
	}

	return false
}

func IsTrue(accessor api.ConditionsAccessor, conditionType string) bool {
	condition := Find(accessor, conditionType)
	return condition != nil && condition.Status == metav1.ConditionTrue
}

func IsFalse(accessor api.ConditionsAccessor, conditionType string) bool {
	condition := Find(accessor, conditionType)
	return condition != nil && condition.Status == metav1.ConditionFalse
}

func IsPresentAndEqual(accessor api.ConditionsAccessor, conditionType string, expected metav1.ConditionStatus) bool {
	condition := Find(accessor, conditionType)
	return condition != nil && condition.Status == expected
}

// Aggregate sets the happy condition from the worst error-severity dependent.
func Aggregate(accessor api.ConditionsAccessor, happyType string, dependentTypes ...string) bool {
	if accessor == nil {
		return false
	}

	var worst *api.Condition

	for _, dependentType := range dependentTypes {
		current := Find(accessor, dependentType)
		if current == nil || current.Severity == api.ConditionSeverityInfo || current.Status == metav1.ConditionTrue {
			continue
		}

		if worst == nil || current.Status == metav1.ConditionFalse && worst.Status == metav1.ConditionUnknown {
			worst = current
		}
	}

	if worst == nil {
		return MarkTrue(accessor, happyType, WithReason("AllDependentsHealthy"))
	}

	return Set(accessor, api.Condition{
		Type:    happyType,
		Status:  worst.Status,
		Reason:  worst.Reason,
		Message: worst.Message,
	})
}

func equal(left, right api.Condition) bool {
	return left.Status == right.Status &&
		left.Reason == right.Reason &&
		left.Message == right.Message &&
		left.Severity == right.Severity &&
		left.ObservedGeneration == right.ObservedGeneration
}

func valueOrZero(value *int64) int64 {
	if value == nil {
		return 0
	}

	return *value
}
