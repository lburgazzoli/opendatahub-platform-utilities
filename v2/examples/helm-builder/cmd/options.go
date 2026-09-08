package main

import (
	"context"
	"errors"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/config"
)

var ErrChartPathRequired = errors.New("helm chart path is required")

const defaultHealthProbeBindAddress = ":8081"

type options struct {
	ChartPath              string
	HealthProbeBindAddress string
}

func loadOptions(ctx context.Context, defaults options) (options, error) {
	environment, err := config.NewEnvironmentSource(
		[]string{"HELM_CHART", "HEALTH_PROBE_BIND_ADDRESS"},
		func(values map[string]string, target *options) error {
			if values["HELM_CHART"] != "" {
				target.ChartPath = values["HELM_CHART"]
			}
			if values["HEALTH_PROBE_BIND_ADDRESS"] != "" {
				target.HealthProbeBindAddress = values["HEALTH_PROBE_BIND_ADDRESS"]
			}

			return nil
		},
	)
	if err != nil {
		return options{}, err
	}

	loader, err := config.New(
		options{
			ChartPath:              defaults.ChartPath,
			HealthProbeBindAddress: defaultHealthProbeBindAddress,
		},
		func(value options) options {
			return value
		},
		environment,
	)
	if err != nil {
		return options{}, err
	}

	loaded, err := loader.Load(ctx)
	if err != nil {
		return options{}, err
	}
	if loaded.ChartPath == "" {
		return options{}, ErrChartPathRequired
	}
	if loaded.HealthProbeBindAddress == "" {
		loaded.HealthProbeBindAddress = defaultHealthProbeBindAddress
	}

	return loaded, nil
}
