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
	switch {
	case gvk == kubegvk.CustomResourceDefinition:
		return 0
	case gvk == kubegvk.Namespace:
		return 10
	case gvk == kubegvk.ServiceAccount:
		return 20
	case gvk == kubegvk.ConfigMap:
		return 20
	case gvk == kubegvk.Secret:
		return 20
	case gvk == kubegvk.Role:
		return 30
	case gvk == kubegvk.RoleBinding:
		return 30
	case gvk == kubegvk.ClusterRole:
		return 30
	case gvk == kubegvk.ClusterRoleBinding:
		return 30
	case gvk == kubegvk.Service:
		return 40
	case gvk == kubegvk.Deployment:
		return 50
	case gvk == kubegvk.StatefulSet:
		return 50
	case gvk == kubegvk.DaemonSet:
		return 50
	case gvk == kubegvk.Job:
		return 50
	case gvk == kubegvk.CronJob:
		return 50
	case gvk == kubegvk.MutatingWebhookConfiguration:
		return 90
	case gvk == kubegvk.ValidatingWebhookConfiguration:
		return 90
	default:
		return 60
	}
}
