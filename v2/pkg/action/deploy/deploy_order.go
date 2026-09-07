package deploy

import (
	"cmp"
	"context"
	"slices"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

// SortFunc orders desired objects before deployment.
type SortFunc func(context.Context, resources.List) (resources.List, error)

// SortFn is retained as a descriptive alias for sorting options.
type SortFn = SortFunc

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
func ApplyOrder(ctx context.Context, objects resources.List) (resources.List, error) {
	_ = ctx
	ordered := append(resources.List(nil), objects...)
	slices.SortStableFunc(ordered, func(left client.Object, right client.Object) int {
		leftRank := applyRank(left.GetObjectKind().GroupVersionKind())
		rightRank := applyRank(right.GetObjectKind().GroupVersionKind())
		return cmp.Compare(leftRank, rightRank)
	})
	return ordered, nil
}

func applyRank(gvk schema.GroupVersionKind) int {
	if rank, found := applyRanks[gvk]; found {
		return rank
	}
	return 60
}
