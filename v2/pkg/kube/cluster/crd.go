package cluster

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
)

// HasCRD reports whether a named CRD exists and is Established. NotFound and
// NoMatch are absence results; other API failures are returned.
func HasCRD(ctx context.Context, reader client.Reader, name string) (bool, error) {
	definition := new(unstructured.Unstructured)
	definition.SetGroupVersionKind(gvk.CustomResourceDefinition)

	err := reader.Get(ctx, client.ObjectKey{Name: name}, definition)
	if err != nil {
		switch {
		case apierrors.IsNotFound(err):
			return false, nil
		case meta.IsNoMatchError(err):
			return false, nil
		default:
			return false, fmt.Errorf("get CRD %q: %w", name, err)
		}
	}

	conditions, found, err := unstructured.NestedSlice(definition.Object, "status", "conditions")
	switch {
	case err != nil:
		return false, fmt.Errorf("read CRD %q conditions: %w", name, err)
	case !found:
		return false, nil
	}

	for _, value := range conditions {
		condition, ok := value.(map[string]any)
		if !ok {
			return false, fmt.Errorf("read CRD %q conditions: condition is not an object", name)
		}

		conditionType, ok := condition["type"].(string)
		if !ok {
			return false, fmt.Errorf("read CRD %q conditions: type is not a string", name)
		}

		conditionStatus, ok := condition["status"].(string)
		if !ok {
			return false, fmt.Errorf("read CRD %q conditions: status is not a string", name)
		}
		if conditionType == "Established" && conditionStatus == "True" {
			return true, nil
		}
	}

	return false, nil
}
