package gc

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

// RunOptions contains values that vary for each GC invocation.
type RunOptions struct {
	Client    client.Client
	Owner     client.Object
	Resources resources.Accessor
}

// ApplyTo copies invocation values into target.
func (o RunOptions) ApplyTo(target *RunOptions) {
	if o.Client != nil {
		target.Client = o.Client
	}
	if o.Owner != nil {
		target.Owner = o.Owner
	}
	if o.Resources != nil {
		target.Resources = o.Resources
	}
}

// RunOption configures one invocation.
type RunOption = option.Option[RunOptions]

func resolveRunOptions(values ...RunOption) (RunOptions, error) {
	var options RunOptions
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}
	if options.Client == nil {
		return RunOptions{}, fmt.Errorf("gc client: %w", ErrRunInputRequired)
	}
	if options.Owner == nil {
		return RunOptions{}, fmt.Errorf("gc owner: %w", ErrRunInputRequired)
	}
	if options.Resources == nil {
		return RunOptions{}, fmt.Errorf("gc resources: %w", ErrRunInputRequired)
	}
	return options, nil
}

var _ RunOption = RunOptions{}
