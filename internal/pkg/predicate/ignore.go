package predicate

import "sigs.k8s.io/controller-runtime/pkg/event"

// IgnoreFuncs is a function that implements Predicate, defaulting to ignore all
// events.
//
// It's the opposite of sigs.k8s.io/controller-runtime/pkg/predicate.Funcs
//
// IgnoreFuncs is intended to be embedded in a controller-specifc Predicate
// struct, with one or more methods overridden.
type IgnoreFuncs struct {
	// Create returns false if the Create event should be processed.
	CreateFunc func(event.CreateEvent) bool

	// Delete returns false if the Delete event should be processed.
	DeleteFunc func(event.DeleteEvent) bool

	// Update returns false if the Update event should be processed.
	UpdateFunc func(event.UpdateEvent) bool

	// Generic returns false if the Generic event should be processed.
	GenericFunc func(event.GenericEvent) bool
}

// Create implements Predicate.
func (p IgnoreFuncs) Create(e event.CreateEvent) bool {
	if p.CreateFunc != nil {
		return p.CreateFunc(e)
	}
	return false
}

// Delete implements Predicate.
func (p IgnoreFuncs) Delete(e event.DeleteEvent) bool {
	if p.DeleteFunc != nil {
		return p.DeleteFunc(e)
	}
	return false
}

// Update implements Predicate.
func (p IgnoreFuncs) Update(e event.UpdateEvent) bool {
	if p.UpdateFunc != nil {
		return p.UpdateFunc(e)
	}
	return false
}

// Generic implements Predicate.
func (p IgnoreFuncs) Generic(e event.GenericEvent) bool {
	if p.GenericFunc != nil {
		return p.GenericFunc(e)
	}
	return false
}
