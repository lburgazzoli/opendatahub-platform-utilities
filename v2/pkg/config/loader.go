package config

import (
	"context"
	"errors"
	"fmt"
	"slices"
)

var (
	ErrNilClone  = errors.New("configuration clone function is required")
	ErrNilSource = errors.New("configuration source is required")
)

// Loader applies immutable source layers over a cloned default value. Sources
// are applied left to right, so later sources have precedence.
type Loader[T any] struct {
	clone    func(T) T
	defaults T
	sources  []Source[T]
}

// New constructs a loader. The clone function is required so every Load call
// starts from independent caller-owned storage.
func New[T any](defaults T, clone func(T) T, sources ...Source[T]) (*Loader[T], error) {
	if clone == nil {
		return nil, ErrNilClone
	}

	for _, source := range sources {
		if source == nil {
			return nil, ErrNilSource
		}
	}

	return &Loader[T]{
		clone:    clone,
		defaults: clone(defaults),
		sources:  slices.Clone(sources),
	}, nil
}

// Load returns a fresh configuration value with all sources applied in order.
func (l *Loader[T]) Load(ctx context.Context) (T, error) {
	value := l.clone(l.defaults)
	for index, source := range l.sources {
		err := source.Apply(ctx, &value)
		if err != nil {
			return value, fmt.Errorf("apply configuration source %d: %w", index, err)
		}
	}

	return l.clone(value), nil
}
