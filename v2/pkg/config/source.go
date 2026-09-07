// Package config composes typed configuration sources without owning a module
// configuration schema.
package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
)

var (
	ErrNilDecoder = errors.New("configuration decoder is required")
	ErrNilApplier = errors.New("configuration source applier is required")
	ErrNilLookup  = errors.New("configuration environment lookup is required")
)

// Source applies one configuration layer to a target value.
type Source[T any] interface {
	Apply(ctx context.Context, target *T) error
}

// SourceFunc adapts a function to Source.
type SourceFunc[T any] func(context.Context, *T) error

// Apply applies the source function.
func (f SourceFunc[T]) Apply(ctx context.Context, target *T) error {
	return f(ctx, target)
}

// FileSource reads one mounted or otherwise supplied file and delegates
// decoding to the consuming module.
type FileSource[T any] struct {
	filesystem fs.FS
	decode     func([]byte, *T) error
	path       string
}

// NewFileSource creates a source backed by an fs.FS. The filesystem and
// decoder are retained immutably; the decoder owns the schema.
func NewFileSource[T any](filesystem fs.FS, path string, decode func([]byte, *T) error) (FileSource[T], error) {
	if filesystem == nil {
		return FileSource[T]{}, fs.ErrInvalid
	}

	if decode == nil {
		return FileSource[T]{}, ErrNilDecoder
	}

	return FileSource[T]{filesystem: filesystem, path: path, decode: decode}, nil
}

// Apply reads and decodes the configured file.
func (s FileSource[T]) Apply(ctx context.Context, target *T) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

	data, err := fs.ReadFile(s.filesystem, s.path)
	if err != nil {
		return fmt.Errorf("read configuration file %q: %w", s.path, err)
	}

	err = s.decode(data, target)
	if err != nil {
		return fmt.Errorf("decode configuration file %q: %w", s.path, err)
	}

	return nil
}

// EnvironmentSource collects configured environment variables and delegates
// schema-specific interpretation to the consuming module.
type EnvironmentSource[T any] struct {
	lookup func(string) (string, bool)
	apply  func(map[string]string, *T) error
	keys   []string
}

// NewEnvironmentSource creates an environment source using os.LookupEnv.
func NewEnvironmentSource[T any](
	keys []string,
	apply func(map[string]string, *T) error,
) (EnvironmentSource[T], error) {
	return NewEnvironmentSourceWithLookup(keys, os.LookupEnv, apply)
}

// NewEnvironmentSourceWithLookup creates an environment source with an
// injectable lookup function for deterministic tests and alternate runtimes.
func NewEnvironmentSourceWithLookup[T any](
	keys []string,
	lookup func(string) (string, bool),
	apply func(map[string]string, *T) error,
) (EnvironmentSource[T], error) {
	if lookup == nil {
		return EnvironmentSource[T]{}, ErrNilLookup
	}

	if apply == nil {
		return EnvironmentSource[T]{}, ErrNilApplier
	}

	return EnvironmentSource[T]{
		keys:   slices.Clone(keys),
		lookup: lookup,
		apply:  apply,
	}, nil
}

// Apply collects present variables and applies them in the declared key order.
func (s EnvironmentSource[T]) Apply(ctx context.Context, target *T) error {
	err := ctx.Err()
	if err != nil {
		return err
	}

	values := make(map[string]string, len(s.keys))
	for _, key := range s.keys {
		if value, ok := s.lookup(key); ok {
			values[key] = value
		}
	}

	err = s.apply(values, target)
	if err != nil {
		return fmt.Errorf("apply environment configuration: %w", err)
	}

	return nil
}
