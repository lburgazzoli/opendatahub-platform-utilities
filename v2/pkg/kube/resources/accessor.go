// Package resources contains Kubernetes resource values and operations.
package resources

import (
	"errors"
	"iter"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrIndexOutOfRange = errors.New("resource index out of range")
	ErrNilTransform    = errors.New("resource transform is required")
)

// List is an ordered collection of Kubernetes objects.
type List []client.Object

// Predicate selects borrowed objects for collection operations.
type Predicate func(client.Object) bool

// TransformFunc returns a replacement object or an error. The input object is
// borrowed and must be treated as read-only.
type TransformFunc func(index int, object client.Object) (client.Object, error)

// Accessor exposes explicit resource collection operations.
type Accessor interface {
	All() iter.Seq2[int, client.Object]
	Len() int
	Get() List
	Set(objects List)
	SetAt(index int, object client.Object) error
	Append(objects ...client.Object)
	Filter(predicate Predicate) int
	Transform(transform TransformFunc) error
}

// Collection is the default ordered resource accessor.
type Collection struct {
	objects List
}

// New creates a collection with a shallow copy of the supplied list.
func New(initial List) *Collection {
	return &Collection{objects: cloneList(initial)}
}

// All iterates over the current collection without allocating.
func (c *Collection) All() iter.Seq2[int, client.Object] {
	return func(yield func(int, client.Object) bool) {
		for index, object := range c.objects {
			if !yield(index, object) {
				return
			}
		}
	}
}

// Len returns the number of objects in the collection.
func (c *Collection) Len() int {
	return len(c.objects)
}

// Get returns a shallow copy of the collection list.
func (c *Collection) Get() List {
	return cloneList(c.objects)
}

// Set replaces the collection list with a shallow copy.
func (c *Collection) Set(objects List) {
	c.objects = cloneList(objects)
}

// SetAt replaces one object without changing collection structure.
func (c *Collection) SetAt(index int, object client.Object) error {
	if index < 0 || index >= len(c.objects) {
		return ErrIndexOutOfRange
	}

	c.objects[index] = object

	return nil
}

// Append adds objects while retaining insertion order and duplicates.
func (c *Collection) Append(objects ...client.Object) {
	c.objects = append(c.objects, objects...)
}

// Filter retains objects whose predicate returns true and returns the number
// discarded.
func (c *Collection) Filter(predicate Predicate) int {
	if predicate == nil {
		return 0
	}

	kept := c.objects[:0]
	discarded := 0

	for _, object := range c.objects {
		if predicate(object) {
			kept = append(kept, object)
			continue
		}

		discarded++
	}

	c.objects = kept

	return discarded
}

// Transform applies replacements to a cloned list and publishes it only when
// every callback succeeds.
func (c *Collection) Transform(transform TransformFunc) error {
	if transform == nil {
		return ErrNilTransform
	}

	transformed := cloneList(c.objects)
	for index, object := range c.objects {
		replacement, err := transform(index, object)
		if err != nil {
			return err
		}

		transformed[index] = replacement
	}

	c.objects = transformed

	return nil
}

func cloneList(objects List) List {
	return append(List(nil), objects...)
}
