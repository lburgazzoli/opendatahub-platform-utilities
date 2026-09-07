// Package resources contains Kubernetes resource values and operations.
package resources

import (
	"errors"
	"iter"
	"slices"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	ErrIndexOutOfRange = errors.New("resource index out of range")
	ErrNilTransform    = errors.New("resource transform is required")
)

// List is an ordered collection of Kubernetes objects.
type List []unstructured.Unstructured

// Predicate selects borrowed objects for collection operations.
type Predicate func(*unstructured.Unstructured) bool

// TransformFunc returns a replacement object or an error. The input object is
// borrowed and must be treated as read-only.
type TransformFunc func(
	index int,
	object *unstructured.Unstructured,
) (unstructured.Unstructured, error)

// Accessor exposes explicit resource collection operations.
type Accessor interface {
	All() iter.Seq2[int, *unstructured.Unstructured]
	Len() int
	Get() List
	Set(objects List)
	SetAt(index int, object unstructured.Unstructured) error
	Append(objects ...unstructured.Unstructured)
	Filter(predicate Predicate) int
	Transform(transform TransformFunc) error
}

// Collection is the default ordered resource accessor.
type Collection struct {
	objects List
}

// New creates a collection with a shallow copy of the supplied list.
func New(initial List) *Collection {
	return &Collection{objects: slices.Clone(initial)}
}

// All iterates over the current collection without allocating.
func (c *Collection) All() iter.Seq2[int, *unstructured.Unstructured] {
	return func(yield func(int, *unstructured.Unstructured) bool) {
		for index := range c.objects {
			if !yield(index, &c.objects[index]) {
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
	return slices.Clone(c.objects)
}

// Set replaces the collection list with a shallow copy.
func (c *Collection) Set(objects List) {
	c.objects = slices.Clone(objects)
}

// SetAt replaces one object without changing collection structure.
func (c *Collection) SetAt(index int, object unstructured.Unstructured) error {
	if index < 0 || index >= len(c.objects) {
		return ErrIndexOutOfRange
	}

	c.objects[index] = object

	return nil
}

// Append adds objects while retaining insertion order and duplicates.
func (c *Collection) Append(objects ...unstructured.Unstructured) {
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

	for index := range c.objects {
		if predicate(&c.objects[index]) {
			kept = append(kept, c.objects[index])
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

	transformed := slices.Clone(c.objects)
	for index := range c.objects {
		replacement, err := transform(index, &c.objects[index])
		if err != nil {
			return err
		}

		transformed[index] = replacement
	}

	c.objects = transformed

	return nil
}
