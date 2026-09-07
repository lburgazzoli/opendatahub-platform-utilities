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

func defaultApplyOrder(ctx context.Context, objects resources.List) (resources.List, error) {
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
	case gvk.Kind == kindNamespace:
		return 10
	case gvk.Kind == "ServiceAccount", gvk.Kind == "ConfigMap", gvk.Kind == "Secret":
		return 20
	case gvk.Kind == "Role", gvk.Kind == "RoleBinding", gvk.Kind == kindClusterRole, gvk.Kind == "ClusterRoleBinding":
		return 30
	case gvk.Kind == "Service":
		return 40
	case gvk.Kind == "Deployment",
		gvk.Kind == "StatefulSet",
		gvk.Kind == "DaemonSet",
		gvk.Kind == "Job",
		gvk.Kind == "CronJob":
		return 50
	case gvk.Kind == "MutatingWebhookConfiguration", gvk.Kind == "ValidatingWebhookConfiguration":
		return 90
	default:
		return 60
	}
}
