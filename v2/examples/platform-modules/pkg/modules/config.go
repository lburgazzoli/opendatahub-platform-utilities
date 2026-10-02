package modules

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
)

// These identifiers match opendatahub-module-deployer's PlatformModuleConfig.
const (
	ConfigAPIVersion = "deployer.opendatahub.io/v1alpha1"
	ConfigKind       = "PlatformModuleConfig"
)

var ErrInvalidConfig = errors.New("invalid module config")

// PlatformModuleConfig is a local YAML descriptor, not a Kubernetes resource.
type PlatformModuleConfig struct {
	APIVersion string         `json:"apiVersion" yaml:"apiVersion"`
	Kind       string         `json:"kind"       yaml:"kind"`
	Metadata   ModuleMetadata `json:"metadata"   yaml:"metadata"`
	Spec       ModuleSpec     `json:"spec"       yaml:"spec"`
}

type ModuleMetadata struct {
	Name string `json:"name" yaml:"name"`
}

type ModuleSpec struct {
	ModuleRef     ModuleRef      `json:"moduleRef"               yaml:"moduleRef"`
	Chart         ChartSpec      `json:"chart,omitzero"          yaml:"chart,omitempty"`
	RelatedImages []string       `json:"relatedImages,omitempty" yaml:"relatedImages,omitempty"`
	Config        map[string]any `json:"config,omitempty"        yaml:"config,omitempty"`
	Services      []string       `json:"services,omitempty"      yaml:"services,omitempty"`
	Runlevel      int            `json:"runlevel,omitzero"       yaml:"runlevel,omitempty"`
}

type ModuleRef struct {
	APIVersion string `json:"apiVersion" yaml:"apiVersion"`
	Kind       string `json:"kind"       yaml:"kind"`
	Name       string `json:"name"       yaml:"name"`
}

type ChartSpec struct {
	Name    string `json:"name,omitempty"    yaml:"name,omitempty"`
	Path    string `json:"path,omitempty"    yaml:"path,omitempty"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// LoadConfigFile reads and validates one deployer-compatible module config.
func LoadConfigFile(path string) (PlatformModuleConfig, error) {
	//nolint:gosec // The caller chooses a local module bundle.
	file, err := os.Open(path)
	if err != nil {
		return PlatformModuleConfig{}, fmt.Errorf("open module config: %w", err)
	}
	defer func() { _ = file.Close() }()

	return LoadConfig(file)
}

// LoadConfig rejects unknown fields and multiple YAML documents.
func LoadConfig(reader io.Reader) (PlatformModuleConfig, error) {
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)

	var config PlatformModuleConfig
	err := decoder.Decode(&config)
	if err != nil {
		return PlatformModuleConfig{}, fmt.Errorf("decode module config: %w", err)
	}

	var extra any
	err = decoder.Decode(&extra)
	if !errors.Is(err, io.EOF) {
		if err == nil {
			return PlatformModuleConfig{}, fmt.Errorf("%w: expected exactly one YAML document", ErrInvalidConfig)
		}

		return PlatformModuleConfig{}, fmt.Errorf("decode trailing YAML: %w", err)
	}

	err = config.Validate()
	if err != nil {
		return PlatformModuleConfig{}, err
	}

	return config, nil
}

// Validate follows the deployer's static PlatformModuleConfig checks.
func (config PlatformModuleConfig) Validate() error {
	if config.APIVersion != ConfigAPIVersion || config.Kind != ConfigKind {
		return fmt.Errorf(
			"%w: expected %s %s, got %s %s",
			ErrInvalidConfig, ConfigAPIVersion, ConfigKind, config.APIVersion, config.Kind,
		)
	}

	if errs := validation.IsDNS1123Subdomain(config.Metadata.Name); len(errs) > 0 {
		return fmt.Errorf("%w: metadata.name %q: %s", ErrInvalidConfig, config.Metadata.Name, strings.Join(errs, ", "))
	}

	_, err := schema.ParseGroupVersion(config.Spec.ModuleRef.APIVersion)
	if err != nil {
		return fmt.Errorf("invalid spec.moduleRef.apiVersion: %w", err)
	}

	if config.Spec.ModuleRef.Kind == "" {
		return fmt.Errorf("%w: spec.moduleRef.kind is required", ErrInvalidConfig)
	}

	if errs := validation.IsDNS1123Subdomain(config.Spec.ModuleRef.Name); len(errs) > 0 {
		return fmt.Errorf(
			"%w: spec.moduleRef.name %q: %s", ErrInvalidConfig, config.Spec.ModuleRef.Name, strings.Join(errs, ", "),
		)
	}

	if config.Spec.Runlevel < 0 {
		return fmt.Errorf("%w: spec.runlevel must be non-negative", ErrInvalidConfig)
	}

	err = uniqueNonEmpty(config.Spec.RelatedImages, "spec.relatedImages")
	if err != nil {
		return err
	}

	return uniqueNonEmpty(config.Spec.Services, "spec.services")
}

func uniqueNonEmpty(values []string, field string) error {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%w: %s contains an empty value", ErrInvalidConfig, field)
		}

		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: %s contains duplicate %q", ErrInvalidConfig, field, value)
		}

		seen[value] = struct{}{}
	}

	return nil
}
