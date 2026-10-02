// Package modules loads the example's declarative module registry.
package modules

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

var (
	ErrInvalidDefinition = errors.New("invalid module definition")
	ErrDuplicateModule   = errors.New("duplicate module")
	ErrEmptyRegistry     = errors.New("empty module registry")
)

// Definition binds a module config to its resolved controller chart.
type Definition struct {
	Chart   string
	CRDName string
	Plural  string
	Config  PlatformModuleConfig
}

func (d Definition) GVK() schema.GroupVersionKind {
	return schema.FromAPIVersionAndKind(d.Config.Spec.ModuleRef.APIVersion, d.Config.Spec.ModuleRef.Kind)
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

		name := definition.Config.Metadata.Name
		if _, exists := registry.definitions[name]; exists {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateModule, name)
		}

		registry.definitions[name] = definition
	}

	if len(registry.definitions) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrEmptyRegistry, root)
	}

	return registry, nil
}

func loadDefinition(directory string) (Definition, error) {
	path := filepath.Join(directory, "module.yaml")
	config, err := LoadConfigFile(path)
	if err != nil {
		return Definition{}, fmt.Errorf("load %q: %w", path, err)
	}

	if config.Metadata.Name != filepath.Base(directory) {
		return Definition{}, fmt.Errorf(
			"%w: metadata.name %q must match directory %q",
			ErrInvalidDefinition,
			config.Metadata.Name,
			filepath.Base(directory),
		)
	}

	chartPath := cmp.Or(config.Spec.Chart.Path, config.Metadata.Name)
	chart, err := resolveChart(directory, chartPath)
	if err != nil {
		return Definition{}, fmt.Errorf("module %q chart: %w", config.Metadata.Name, err)
	}

	crd, err := readChartCRD(chart, config)
	if err != nil {
		return Definition{}, err
	}

	return Definition{
		Chart:   chart,
		CRDName: crd.Name,
		Plural:  crd.Spec.Names.Plural,
		Config:  config,
	}, nil
}

func resolveChart(directory string, chartPath string) (string, error) {
	if !filepath.IsLocal(chartPath) {
		return "", fmt.Errorf("%w: chart path %q must be local", ErrInvalidDefinition, chartPath)
	}

	root, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return "", fmt.Errorf("resolve module directory %q: %w", directory, err)
	}

	chart, err := filepath.EvalSymlinks(filepath.Join(root, chartPath))
	if err != nil {
		return "", fmt.Errorf("resolve chart path %q: %w", chartPath, err)
	}

	relative, err := filepath.Rel(root, chart)
	if err != nil || !filepath.IsLocal(relative) && relative != "." {
		return "", fmt.Errorf("%w: chart path %q escapes module directory", ErrInvalidDefinition, chartPath)
	}

	_, err = os.Stat(filepath.Join(chart, "Chart.yaml"))
	if err != nil {
		return "", fmt.Errorf("chart metadata: %w", err)
	}

	return chart, nil
}

func readChartCRD(chart string, config PlatformModuleConfig) (apiextensionsv1.CustomResourceDefinition, error) {
	//nolint:gosec // The path is selected from the local module registry.
	data, err := os.ReadFile(filepath.Join(chart, "charts", "module", "templates", "crd.yaml"))
	if err != nil {
		return apiextensionsv1.CustomResourceDefinition{}, fmt.Errorf("module %q CRD: %w", config.Metadata.Name, err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	err = yaml.UnmarshalStrict(data, &crd)
	if err != nil {
		return apiextensionsv1.CustomResourceDefinition{}, fmt.Errorf("decode module %q CRD: %w", config.Metadata.Name, err)
	}

	gvk := schema.FromAPIVersionAndKind(config.Spec.ModuleRef.APIVersion, config.Spec.ModuleRef.Kind)
	if crd.Spec.Group != gvk.Group || crd.Spec.Names.Kind != gvk.Kind || crd.Name != crd.Spec.Names.Plural+"."+gvk.Group {
		return apiextensionsv1.CustomResourceDefinition{}, fmt.Errorf(
			"%w: module %q CRD does not match spec.moduleRef", ErrInvalidDefinition, config.Metadata.Name,
		)
	}

	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Name != gvk.Version {
		return apiextensionsv1.CustomResourceDefinition{}, fmt.Errorf(
			"%w: module %q CRD version does not match spec.moduleRef", ErrInvalidDefinition, config.Metadata.Name,
		)
	}

	return crd, nil
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
