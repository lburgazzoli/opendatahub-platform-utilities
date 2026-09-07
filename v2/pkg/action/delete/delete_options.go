package deletion

import (
	"maps"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

// Options configures the delete action.
type Options struct {
	Labels    map[string]string
	Namespace string
	DeleteAll bool
}

// ApplyTo copies delete configuration.
func (o Options) ApplyTo(target *Options) {
	target.Labels = maps.Clone(o.Labels)
	target.Namespace = o.Namespace
	target.DeleteAll = o.DeleteAll
}

// Option configures an action.
type Option = option.Option[Options]

// WithLabels replaces the action's label selector.
func WithLabels(values map[string]string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Labels = maps.Clone(values)
	})
}

// WithNamespace supplies the default namespace for namespaced resources.
func WithNamespace(namespace string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Namespace = namespace
	})
}

// WithDeleteAll explicitly permits an unbounded deletion selector.
func WithDeleteAll() Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.DeleteAll = true
	})
}

var _ Option = Options{}
