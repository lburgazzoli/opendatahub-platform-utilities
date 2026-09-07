package pipeline

import (
	"slices"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

type Options struct {
	Before  []Registration
	Main    []Registration
	After   []Registration
	Cleanup []Registration
}

func (o Options) ApplyTo(target *Options) {
	if o.Before != nil {
		target.Before = slices.Clone(o.Before)
	}

	if o.Main != nil {
		target.Main = slices.Clone(o.Main)
	}

	if o.After != nil {
		target.After = slices.Clone(o.After)
	}

	if o.Cleanup != nil {
		target.Cleanup = slices.Clone(o.Cleanup)
	}
}

type Option = option.Option[Options]
