package predicate

import (
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DependentOptions controls updates accepted for a dependent resource.
type DependentOptions struct {
	WatchDelete bool
	WatchUpdate bool
	WatchStatus bool
}

// Dependent accepts meaningful dependent-resource changes while ignoring
// status-only updates unless explicitly requested.
func Dependent(options DependentOptions) crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return options.WatchDelete },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			if !options.WatchUpdate || value.ObjectOld == nil || value.ObjectNew == nil {
				return false
			}

			if value.ObjectOld.GetResourceVersion() == value.ObjectNew.GetResourceVersion() {
				return false
			}

			oldObject, err := comparableObject(value.ObjectOld, options.WatchStatus)
			if err != nil {
				return true
			}

			newObject, err := comparableObject(value.ObjectNew, options.WatchStatus)
			if err != nil {
				return true
			}

			return !equality.Semantic.DeepEqual(oldObject, newObject)
		},
	}
}

// HashChanged accepts updates where the object content changed after removing
// server-owned metadata.
func HashChanged() crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			if value.ObjectOld == nil || value.ObjectNew == nil {
				return false
			}

			oldObject, err := comparableObject(value.ObjectOld, true)
			if err != nil {
				return true
			}

			newObject, err := comparableObject(value.ObjectNew, true)
			if err != nil {
				return true
			}

			return !equality.Semantic.DeepEqual(oldObject, newObject)
		},
	}
}

func comparableObject(object client.Object, watchStatus bool) (map[string]any, error) {
	converted, err := resources.ToUnstructured(object)
	if err != nil {
		return nil, err
	}

	if !watchStatus {
		unstructured.RemoveNestedField(converted.Object, "status")
	}

	// Ignore metadata that changes without changing the object's meaningful content.
	unstructured.RemoveNestedField(converted.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(converted.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(converted.Object, "metadata", "annotations", corev1.LastAppliedConfigAnnotation)

	metadata, found, err := unstructured.NestedMap(converted.Object, "metadata")
	if err != nil {
		return nil, err
	}

	// Treat empty and absent metadata as equivalent after the ignored fields are removed.
	if found && len(metadata) == 0 {
		unstructured.RemoveNestedField(converted.Object, "metadata")
	}

	return converted.Object, nil
}
