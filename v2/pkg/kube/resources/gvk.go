package resources

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// GvkToUnstructured creates an empty unstructured object with the given GVK.
func GvkToUnstructured(gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)

	return u
}

// GvkToPartial creates a metadata-only object with the given GVK.
func GvkToPartial(gvk schema.GroupVersionKind) *metav1.PartialObjectMetadata {
	partial := &metav1.PartialObjectMetadata{}
	partial.SetGroupVersionKind(gvk)

	return partial
}
