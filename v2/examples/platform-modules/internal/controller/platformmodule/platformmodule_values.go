package platformmodule

import (
	"fmt"

	"github.com/go-viper/mapstructure/v2"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
)

// ChartValues contains the values passed to a module's umbrella chart.
type ChartValues struct {
	Module      ModuleValues     `mapstructure:"module"`
	Projections ProjectionValues `mapstructure:"projections"`
}

// ModuleValues contains a module definition and its runtime settings.
type ModuleValues struct {
	Namespace  string             `mapstructure:"namespace"`
	Image      string             `mapstructure:"image"`
	ModuleSpec modules.ModuleSpec `mapstructure:",squash"`
	Enabled    bool               `mapstructure:"enabled"`
}

// ProjectionValues controls the projections chart.
type ProjectionValues struct {
	Enabled bool `mapstructure:"enabled"`
}

// ToValues converts a typed value to the renderer's values type.
func ToValues(value any) (manifesttypes.Values, error) {
	values := make(manifesttypes.Values)
	err := mapstructure.Decode(value, &values)
	if err != nil {
		return nil, fmt.Errorf("convert manifest values: %w", err)
	}

	return values, nil
}
