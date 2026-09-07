package gc

import (
	"context"
	"reflect"

	authorizationv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func canDelete(
	ctx context.Context,
	cli client.Client,
	mapping *meta.RESTMapping,
	namespace string,
) (bool, error) {
	review := &authorizationv1.SelfSubjectAccessReview{
		Spec: authorizationv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authorizationv1.ResourceAttributes{
				Group:     mapping.Resource.Group,
				Version:   mapping.Resource.Version,
				Resource:  mapping.Resource.Resource,
				Verb:      "delete",
				Namespace: namespace,
			},
		},
	}

	err := cli.Create(ctx, review)
	if err != nil {
		return false, err
	}

	return review.Status.Allowed, nil
}

func gcNamespace(
	configured *string,
	owner client.Object,
	mapping *meta.RESTMapping,
) string {
	if mapping.Scope.Name() != meta.RESTScopeNameNamespace {
		return ""
	}
	if configured != nil {
		return *configured
	}

	return owner.GetNamespace()
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}

	valueOf := reflect.ValueOf(value)
	switch valueOf.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return valueOf.IsNil()
	default:
		return false
	}
}
