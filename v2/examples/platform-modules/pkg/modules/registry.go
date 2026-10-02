// Package modules loads the example's declarative module registry.
package modules

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var (
	ErrInvalidDefinition = errors.New("invalid module definition")
	ErrDuplicateModule   = errors.New("duplicate module")
	ErrEmptyRegistry     = errors.New("empty module registry")
)

// Definition binds a module name to its CRD and controller chart.
type Definition struct {
	Values     map[string]any `json:"values,omitempty"`
	Name       string         `json:"name"`
	CRD        string         `json:"crd"`
	APIVersion string         `json:"apiVersion"`
	Kind       string         `json:"kind"`
	Chart      string         `json:"chart"`
}

func (d Definition) GVK() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(d.APIVersion, d.Kind)
}

func (d Definition) Resource() string {
	resource, _, _ := strings.Cut(d.CRD, ".")

	return resource
}

// Registry is an immutable startup snapshot of module bundles.
type Registry struct {
	definitions map[string]Definition
}

// Load reads one module.yaml from every immediate subdirectory of root.
func Load(root string) (*Registry, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("read module directory %q: %w", root, err)
	}

	registry := &Registry{definitions: make(map[string]Definition)}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		definition, err := loadDefinition(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}

		if _, exists := registry.definitions[definition.Name]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateModule, definition.Name)
		}

		registry.definitions[definition.Name] = definition
	}

	if len(registry.definitions) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrEmptyRegistry, root)
	}

	return registry, nil
}

func loadDefinition(directory string) (Definition, error) {
	path := filepath.Join(directory, "module.yaml")
	data, err := os.ReadFile(path) //nolint:gosec // Paths come from entries in the selected registry directory.
	if err != nil {
		return Definition{}, fmt.Errorf("read %q: %w", path, err)
	}

	var definition Definition
	err = yaml.UnmarshalStrict(data, &definition)
	if err != nil {
		return Definition{}, fmt.Errorf("decode %q: %w", path, err)
	}

	err = definition.validate()
	if err != nil {
		return Definition{}, fmt.Errorf("validate %q: %w", path, err)
	}

	definition.Chart = filepath.Clean(filepath.Join(directory, definition.Chart))
	_, err = os.Stat(filepath.Join(definition.Chart, "Chart.yaml"))
	if err != nil {
		return Definition{}, fmt.Errorf("module %q chart: %w", definition.Name, err)
	}

	return definition, nil
}

func (d Definition) validate() error {
	if slices.Contains([]string{d.Name, d.CRD, d.APIVersion, d.Kind, d.Chart}, "") {
		return fmt.Errorf("%w: name, crd, apiVersion, kind, and chart are required", ErrInvalidDefinition)
	}

	groupVersion, err := schema.ParseGroupVersion(d.APIVersion)
	if err != nil {
		return fmt.Errorf("apiVersion %q: %w", d.APIVersion, err)
	}

	resource, group, found := strings.Cut(d.CRD, ".")
	if !found || resource == "" || group != groupVersion.Group {
		return fmt.Errorf("%w: crd %q must use group %q", ErrInvalidDefinition, d.CRD, groupVersion.Group)
	}

	return nil
}

func (r *Registry) Get(name string) (Definition, bool) {
	definition, found := r.definitions[name]

	return definition, found
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	slices.Sort(names)

	return names
}
