// Package singleton contains singleton admission helpers.
package singleton

import (
	"context"
	"fmt"
	"net/http"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidateCreation denies a CREATE request when an instance already exists.
func ValidateCreation(
	ctx context.Context,
	reader client.Reader,
	request *admission.Request,
	gvk schema.GroupVersionKind,
) admission.Response {
	if request.Operation != admissionv1.Create {
		return admission.Allowed("singleton validation only applies to CREATE")
	}

	count, err := Count(ctx, reader, gvk)
	if err != nil {
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if count > 0 {
		return admission.Denied(fmt.Sprintf("only one instance of %s is allowed", gvk.Kind))
	}

	return admission.Allowed("")
}

// Count counts objects of a GVK using an unstructured list.
func Count(ctx context.Context, reader client.Reader, gvk schema.GroupVersionKind) (int, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	err := reader.List(ctx, list)
	switch {
	case apierrors.IsNotFound(err), meta.IsNoMatchError(err):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("count %s objects: %w", gvk, err)
	default:
		return len(list.Items), nil
	}
}
