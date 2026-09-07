package pipeline

import (
	"slices"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

type ActionOptions struct {
	Name   string
	Guards []Guard
}

func (o ActionOptions) ApplyTo(target *ActionOptions) {
	if o.Name != "" {
		target.Name = o.Name
	}

	if o.Guards != nil {
		target.Guards = slices.Clone(o.Guards)
	}
}

type ActionOption = option.Option[ActionOptions]

func WithName(name string) ActionOption {
	return option.FunctionalOption[ActionOptions](func(options *ActionOptions) {
		options.Name = name
	})
}

func When(guard Guard) ActionOption {
	return option.FunctionalOption[ActionOptions](func(options *ActionOptions) {
		options.Guards = append(options.Guards, guard)
	})
}
