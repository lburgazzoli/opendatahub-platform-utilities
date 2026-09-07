package deploy

import (
	"cmp"
	"slices"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

// SortFunc orders desired objects before deployment in place.
type SortFunc func(resources.List)

const defaultApplyRank = 60

//nolint:gochecknoglobals // The rank table is immutable package configuration.
var applyRanks = map[schema.GroupVersionKind]int{
	kubegvk.CustomResourceDefinition:       0,
	kubegvk.Namespace:                      10,
	kubegvk.ServiceAccount:                 20,
	kubegvk.ConfigMap:                      20,
	kubegvk.Secret:                         20,
	kubegvk.Role:                           30,
	kubegvk.RoleBinding:                    30,
	kubegvk.ClusterRole:                    30,
	kubegvk.ClusterRoleBinding:             30,
	kubegvk.Service:                        40,
	kubegvk.Deployment:                     50,
	kubegvk.StatefulSet:                    50,
	kubegvk.DaemonSet:                      50,
	kubegvk.Job:                            50,
	kubegvk.CronJob:                        50,
	kubegvk.MutatingWebhookConfiguration:   90,
	kubegvk.ValidatingWebhookConfiguration: 90,
}

// ApplyOrder orders desired objects by Kubernetes dependency rank.
func ApplyOrder(objects resources.List) {
	slices.SortStableFunc(objects, func(left client.Object, right client.Object) int {
		leftRank, leftFound := applyRanks[left.GetObjectKind().GroupVersionKind()]
		if !leftFound {
			leftRank = defaultApplyRank
		}

		rightRank, rightFound := applyRanks[right.GetObjectKind().GroupVersionKind()]
		if !rightFound {
			rightRank = defaultApplyRank
		}

		return cmp.Compare(leftRank, rightRank)
	})
}
