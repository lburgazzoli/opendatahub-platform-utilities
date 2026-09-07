package deployment

import (
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// RunOptions contains values that vary for one observation.
type RunOptions struct {
	Client    client.Reader
	Namespace string
}

// ApplyTo copies invocation values.
func (o RunOptions) ApplyTo(target *RunOptions) {
	target.Client = o.Client
	target.Namespace = o.Namespace
}

// RunOption configures one observation invocation.
type RunOption = option.Option[RunOptions]

func resolveRunOptions(values ...RunOption) (RunOptions, error) {
	var options RunOptions
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	if options.Client == nil {
		return RunOptions{}, ErrClientRequired
	}

	return options, nil
}

var _ RunOption = RunOptions{}
