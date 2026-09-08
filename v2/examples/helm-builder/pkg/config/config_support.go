package config

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

func applyFiles(ctx context.Context, filesystem fs.FS, target *Config) error {
	if filesystem == nil {
		return nil
	}

	settings := []string{
		ChartPathConfigKey,
		NamespaceConfigKey,
		HealthProbeBindAddressConfigKey,
	}

	for _, setting := range settings {
		err := ctx.Err()
		if err != nil {
			return err
		}

		data, err := fs.ReadFile(filesystem, setting)
		switch {
		case err == nil:
			value := strings.TrimSpace(string(data))
			switch setting {
			case ChartPathConfigKey:
				target.ChartPath = value
			case NamespaceConfigKey:
				target.Namespace = value
			case HealthProbeBindAddressConfigKey:
				target.HealthProbeBindAddress = value
			}
		case errors.Is(err, fs.ErrNotExist):
			continue
		default:
			return fmt.Errorf("read configuration file %q: %w", setting, err)
		}
	}

	return nil
}
