package predicate

import "sigs.k8s.io/controller-runtime/pkg/event"

// Funcs implements a predicate with false defaults for unspecified events.
type Funcs struct {
	CreateFunc  func(event.CreateEvent) bool
	DeleteFunc  func(event.DeleteEvent) bool
	GenericFunc func(event.GenericEvent) bool
	UpdateFunc  func(event.UpdateEvent) bool
}

func (p Funcs) Create(value event.CreateEvent) bool {
	if p.CreateFunc == nil {
		return false
	}

	return p.CreateFunc(value)
}

func (p Funcs) Delete(value event.DeleteEvent) bool {
	if p.DeleteFunc == nil {
		return false
	}

	return p.DeleteFunc(value)
}

func (p Funcs) Generic(value event.GenericEvent) bool {
	if p.GenericFunc == nil {
		return false
	}

	return p.GenericFunc(value)
}

func (p Funcs) Update(value event.UpdateEvent) bool {
	if p.UpdateFunc == nil {
		return false
	}

	return p.UpdateFunc(value)
}
