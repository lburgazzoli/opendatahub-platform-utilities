package predicate

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DeleteEvent accepts delete events only.
func DeleteEvent() crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return true },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc:  func(event.UpdateEvent) bool { return false },
	}
}

// GenerationChanged accepts creates and updates that change generation.
func GenerationChanged() crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return true },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			if value.ObjectOld == nil || value.ObjectNew == nil {
				return false
			}

			if value.ObjectOld.GetGeneration() == 0 || value.ObjectNew.GetGeneration() == 0 {
				return true
			}

			return value.ObjectOld.GetGeneration() != value.ObjectNew.GetGeneration()
		},
	}
}

// PartialOptions controls which partial-object events are accepted.
type PartialOptions struct {
	WatchDelete bool
	WatchUpdate bool
}

// Partial accepts resource-version changes for PartialObjectMetadata events.
func Partial(options PartialOptions) crpredicate.Predicate {
	return Funcs{
		CreateFunc: func(event.CreateEvent) bool { return false },
		DeleteFunc: func(value event.DeleteEvent) bool {
			return options.WatchDelete && isPartial(value.Object)
		},
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			if !options.WatchUpdate || value.ObjectOld == nil || value.ObjectNew == nil {
				return false
			}

			return value.ObjectOld.GetResourceVersion() != value.ObjectNew.GetResourceVersion() &&
				isPartial(value.ObjectNew)
		},
	}
}

func isPartial(object any) bool {
	_, ok := object.(*metav1.PartialObjectMetadata)
	return ok
}
