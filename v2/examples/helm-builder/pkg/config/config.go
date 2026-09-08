package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	platformconfig "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/config"
)

const (
	ConfigurationPathEnvVar         = "HELM_EXAMPLE_CONFIGURATION_PATH"
	ChartPathEnvVar                 = "HELM_EXAMPLE_CHART"
	NamespaceEnvVar                 = "HELM_EXAMPLE_NAMESPACE"
	HealthProbeBindAddressEnvVar    = "HELM_EXAMPLE_HEALTH_PROBE_BIND_ADDRESS"
	ChartPathConfigKey              = "chart-path"
	NamespaceConfigKey              = "namespace"
	HealthProbeBindAddressConfigKey = "controller.health.bind-address"
	DefaultHealthProbeBindAddress   = ":8081"
)

var (
	ErrChartPathRequired = errors.New("helm chart path is required")
	ErrNamespaceRequired = errors.New("target namespace is required")
)

// Config contains immutable startup configuration for the example.
type Config struct {
	// ChartPath is the filesystem path to the Helm chart.
	ChartPath string
	// Namespace is the namespace watched and used by the example controller.
	Namespace string
	// HealthProbeBindAddress is the controller-runtime health endpoint address.
	HealthProbeBindAddress string
}

// Load reads configuration from an optional mounted configuration directory
// and then applies environment variable overrides.
func Load(ctx context.Context) (*Config, error) {
	var filesystem fs.FS

	configurationPath := os.Getenv(ConfigurationPathEnvVar)
	if configurationPath != "" {
		filesystem = os.DirFS(configurationPath)
	}

	return LoadFromFS(ctx, filesystem)
}

// LoadFromFS loads configuration from defaults, an optional filesystem, and
// environment variables in increasing order of precedence.
func LoadFromFS(ctx context.Context, filesystem fs.FS) (*Config, error) {
	fileSource := platformconfig.SourceFunc[Config](func(ctx context.Context, target *Config) error {
		return applyFiles(ctx, filesystem, target)
	})

	environment, err := platformconfig.NewEnvironmentSource(
		[]string{ChartPathEnvVar, NamespaceEnvVar, HealthProbeBindAddressEnvVar},
		func(values map[string]string, target *Config) error {
			if value, ok := values[ChartPathEnvVar]; ok {
				target.ChartPath = value
			}
			if value, ok := values[NamespaceEnvVar]; ok {
				target.Namespace = value
			}
			if value, ok := values[HealthProbeBindAddressEnvVar]; ok {
				target.HealthProbeBindAddress = value
			}

			return nil
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create environment source: %w", err)
	}

	loader, err := platformconfig.New(
		Config{HealthProbeBindAddress: DefaultHealthProbeBindAddress},
		func(value Config) Config {
			return value
		},
		fileSource,
		environment,
	)
	if err != nil {
		return nil, fmt.Errorf("create configuration loader: %w", err)
	}

	loaded, err := loader.Load(ctx)
	if err != nil {
		return nil, err
	}

	err = loaded.Validate()
	if err != nil {
		return nil, err
	}

	return &loaded, nil
}

// Validate checks required startup configuration.
func (c Config) Validate() error {
	if c.ChartPath == "" {
		return ErrChartPathRequired
	}
	if c.Namespace == "" {
		return ErrNamespaceRequired
	}

	return nil
}
