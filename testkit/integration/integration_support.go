package integration

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

func dataScienceClusterGVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "datasciencecluster.opendatahub.io",
		Version: "v2",
		Kind:    "DataScienceCluster",
	}
}

type statusConditionAccessor struct {
	conditions []platformapi.Condition
}

func (a *statusConditionAccessor) GetConditions() []platformapi.Condition {
	return a.conditions
}

func (a *statusConditionAccessor) SetConditions(conditions []platformapi.Condition) {
	a.conditions = conditions
}

var _ platformapi.ConditionsAccessor = (*statusConditionAccessor)(nil)
