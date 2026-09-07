package deployment

import (
	"maps"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

// Options configures deployment observation.
type Options struct {
	Labels             map[string]string
	ConditionType      string
	NotAvailableReason string
}

// ApplyTo copies deployment options.
func (o Options) ApplyTo(target *Options) {
	target.Labels = maps.Clone(o.Labels)
	target.ConditionType = o.ConditionType
	target.NotAvailableReason = o.NotAvailableReason
}

// Option configures a deployment action.
type Option = option.Option[Options]

// WithSelectorLabels replaces the deployment selector.
func WithSelectorLabels(values map[string]string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Labels = maps.Clone(values)
	})
}

// WithConditionType changes the reported condition type.
func WithConditionType(value string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ConditionType = value
	})
}

// WithNotAvailableReason changes the false-condition reason.
func WithNotAvailableReason(value string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.NotAvailableReason = value
	})
}

var _ Option = Options{}
