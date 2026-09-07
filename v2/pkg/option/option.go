// Package option contains the dependency-free generic option primitive.
package option

// Option applies configuration to a complete value.
type Option[T any] interface {
	ApplyTo(target *T)
}

// FunctionalOption adapts a function to Option.
type FunctionalOption[T any] func(*T)

// ApplyTo applies the functional option.
func (f FunctionalOption[T]) ApplyTo(target *T) {
	f(target)
}
