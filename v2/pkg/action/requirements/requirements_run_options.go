package requirements

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

// RunOptions contains values that vary for each requirement check.
type RunOptions struct {
	Client             client.Client
	Conditions         api.ConditionsAccessor
	ObservedGeneration int64
}

// ApplyTo copies invocation values into target.
func (o RunOptions) ApplyTo(target *RunOptions) {
	target.Client = o.Client
	target.Conditions = o.Conditions
	target.ObservedGeneration = o.ObservedGeneration
}

// RunOption configures one requirement invocation.
type RunOption = option.Option[RunOptions]

// WithClient supplies the Kubernetes client.
func WithClient(cli client.Client) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		options.Client = cli
	})
}

// WithConditions supplies the condition capability owned by the instance.
func WithConditions(conditions api.ConditionsAccessor) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		options.Conditions = conditions
	})
}

// WithObservedGeneration supplies the generation used by the condition.
func WithObservedGeneration(generation int64) RunOption {
	return option.FunctionalOption[RunOptions](func(options *RunOptions) {
		options.ObservedGeneration = generation
	})
}

func resolveRunOptions(values ...RunOption) (RunOptions, error) {
	var options RunOptions
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}
	if options.Client == nil {
		return RunOptions{}, fmt.Errorf("requirements client: %w", ErrClientRequired)
	}
	if options.Conditions == nil {
		return RunOptions{}, fmt.Errorf("requirements conditions: %w", ErrConditionsRequired)
	}

	return options, nil
}

var _ RunOption = RunOptions{}
