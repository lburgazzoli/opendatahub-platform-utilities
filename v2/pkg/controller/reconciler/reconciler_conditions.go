package reconciler

import (
	"slices"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

func aggregateConditions(accessor api.ConditionsAccessor) {
	dependentTypes := make([]string, 0, len(accessor.GetConditions()))
	for _, current := range accessor.GetConditions() {
		if current.Type == string(api.ConditionTypeReady) {
			continue
		}

		dependentTypes = append(dependentTypes, current.Type)
	}

	condition.Aggregate(
		accessor,
		string(api.ConditionTypeReady),
		slices.Compact(dependentTypes)...,
	)
}
