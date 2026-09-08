package reconciler

import (
	"slices"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

// ConditionManager processes framework-owned condition state after the action
// pipeline has completed.
type ConditionManager interface {
	Apply(outcome action.ActionError, generation int64)
}

// ConditionManagerFactory creates a manager for one reconciliation status.
type ConditionManagerFactory func(accessor api.ConditionsAccessor) ConditionManager

type defaultConditionManager struct {
	accessor api.ConditionsAccessor
}

func defaultConditionManagerFactory(accessor api.ConditionsAccessor) ConditionManager {
	return &defaultConditionManager{accessor: accessor}
}

func (m *defaultConditionManager) Apply(outcome action.ActionError, generation int64) {
	markProvisioning(m.accessor, outcome, generation)

	dependentTypes := make([]string, 0, len(m.accessor.GetConditions()))
	for _, current := range m.accessor.GetConditions() {
		if current.Type == string(api.ConditionTypeReady) {
			continue
		}

		dependentTypes = append(dependentTypes, current.Type)
	}

	condition.Aggregate(
		m.accessor,
		string(api.ConditionTypeReady),
		slices.Compact(dependentTypes)...,
	)
}
