package predicate

import (
	"slices"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// AnnotationChanged accepts creates and updates where the selected annotation changed.
func AnnotationChanged(name string) crpredicate.Predicate {
	return metadataChanged(name, func(object client.Object) map[string]string {
		return object.GetAnnotations()
	})
}

// HasAnnotation accepts events where the selected annotation has one of the values.
func HasAnnotation(name string, values ...string) crpredicate.Predicate {
	return metadataHas(name, values, func(object client.Object) map[string]string {
		return object.GetAnnotations()
	})
}

// LabelChanged accepts creates and updates where the selected label changed.
func LabelChanged(name string) crpredicate.Predicate {
	return metadataChanged(name, func(object client.Object) map[string]string {
		return object.GetLabels()
	})
}

// HasLabel accepts events where the selected label has one of the values.
func HasLabel(name string, values ...string) crpredicate.Predicate {
	return metadataHas(name, values, func(object client.Object) map[string]string {
		return object.GetLabels()
	})
}

func metadataChanged(
	name string,
	values func(client.Object) map[string]string,
) crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			if value.ObjectOld == nil || value.ObjectNew == nil {
				return false
			}

			return values(value.ObjectOld)[name] != values(value.ObjectNew)[name]
		},
	}
}

func metadataHas(
	name string,
	expected []string,
	values func(client.Object) map[string]string,
) crpredicate.Predicate {
	return Funcs{
		CreateFunc: func(value event.CreateEvent) bool {
			return value.Object != nil && metadataMatches(values(value.Object), name, expected)
		},
		DeleteFunc: func(value event.DeleteEvent) bool {
			return value.Object != nil && metadataMatches(values(value.Object), name, expected)
		},
		GenericFunc: func(value event.GenericEvent) bool {
			return value.Object != nil && metadataMatches(values(value.Object), name, expected)
		},
		UpdateFunc: func(value event.UpdateEvent) bool {
			if value.ObjectOld == nil || value.ObjectNew == nil {
				return false
			}

			return metadataMatches(values(value.ObjectOld), name, expected) ||
				metadataMatches(values(value.ObjectNew), name, expected)
		},
	}
}

func metadataMatches(values map[string]string, name string, expected []string) bool {
	value, found := values[name]
	if len(expected) == 0 {
		return found
	}

	return found && slices.Contains(expected, value)
}
