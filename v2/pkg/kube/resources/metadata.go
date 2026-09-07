package resources

import (
	"maps"
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SetLabels merges labels onto an object.
func SetLabels(object client.Object, values map[string]string) {
	if object == nil || len(values) == 0 {
		return
	}

	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string, len(values))
	}

	maps.Copy(labels, values)
	object.SetLabels(labels)
}

// SetAnnotations merges annotations onto an object.
func SetAnnotations(object client.Object, values map[string]string) {
	if object == nil || len(values) == 0 {
		return
	}

	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string, len(values))
	}

	maps.Copy(annotations, values)
	object.SetAnnotations(annotations)
}

// GetLabel returns a label value or an empty string.
func GetLabel(object client.Object, key string) string {
	if object == nil {
		return ""
	}

	return object.GetLabels()[key]
}

// GetAnnotation returns an annotation value or an empty string.
func GetAnnotation(object client.Object, key string) string {
	if object == nil {
		return ""
	}

	return object.GetAnnotations()[key]
}

// HasAnnotation reports whether an annotation exists. When values are
// supplied, it reports whether the annotation value is one of them.
func HasAnnotation(object client.Object, key string, values ...string) bool {
	if object == nil {
		return false
	}

	value, ok := object.GetAnnotations()[key]
	if len(values) == 0 {
		return ok
	}

	return ok && slices.Contains(values, value)
}
