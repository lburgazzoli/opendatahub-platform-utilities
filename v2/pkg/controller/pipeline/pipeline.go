package pipeline

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
)

type Pipeline struct {
	options  Options
	validate func() error
}

func New(options ...Option) *Pipeline {
	configured := Options{}

	for _, option := range options {
		if option != nil {
			option.ApplyTo(&configured)
		}
	}

	configuredPipeline := &Pipeline{options: Options{
		Before:  slices.Clone(configured.Before),
		Main:    slices.Clone(configured.Main),
		After:   slices.Clone(configured.After),
		Cleanup: slices.Clone(configured.Cleanup),
	}}
	configuredPipeline.validate = sync.OnceValue(configuredPipeline.validateOptions)

	return configuredPipeline
}

func (p *Pipeline) WithBeforeAction(actionValue Action, options ...ActionOption) *Pipeline {
	return p.withRegistration(func(configured *Options, registration Registration) {
		configured.Before = append(configured.Before, registration)
	}, Register(actionValue, options...))
}

func (p *Pipeline) WithAction(actionValue Action, options ...ActionOption) *Pipeline {
	return p.withRegistration(func(configured *Options, registration Registration) {
		configured.Main = append(configured.Main, registration)
	}, Register(actionValue, options...))
}

func (p *Pipeline) WithAfterAction(actionValue Action, options ...ActionOption) *Pipeline {
	return p.withRegistration(func(configured *Options, registration Registration) {
		configured.After = append(configured.After, registration)
	}, Register(actionValue, options...))
}

func (p *Pipeline) WithCleanupAction(actionValue Action, options ...ActionOption) *Pipeline {
	return p.withRegistration(func(configured *Options, registration Registration) {
		configured.Cleanup = append(configured.Cleanup, registration)
	}, Register(actionValue, options...))
}

func (p *Pipeline) WithBeforeActionFunc(
	execute func(context.Context, *Request) error,
	options ...ActionOption,
) *Pipeline {
	return p.WithBeforeAction(Wrap(execute), options...)
}

func (p *Pipeline) WithActionFunc(execute func(context.Context, *Request) error, options ...ActionOption) *Pipeline {
	return p.WithAction(Wrap(execute), options...)
}

func (p *Pipeline) WithAfterActionFunc(
	execute func(context.Context, *Request) error,
	options ...ActionOption,
) *Pipeline {
	return p.WithAfterAction(Wrap(execute), options...)
}

func (p *Pipeline) WithCleanupActionFunc(
	execute func(context.Context, *Request) error,
	options ...ActionOption,
) *Pipeline {
	return p.WithCleanupAction(Wrap(execute), options...)
}

// HasCleanupActions reports whether the pipeline has deletion cleanup actions.
func (p *Pipeline) HasCleanupActions() bool {
	return len(p.options.Cleanup) > 0
}

func (p *Pipeline) Validate() error {
	return p.validate()
}

func (p *Pipeline) validateOptions() error {
	seen := make(map[string]string)
	phases := []struct {
		name string
		list []Registration
	}{
		{name: "before", list: p.options.Before},
		{name: "main", list: p.options.Main},
		{name: "after", list: p.options.After},
		{name: "cleanup", list: p.options.Cleanup},
	}

	for _, phase := range phases {
		for position, registration := range phase.list {
			err := registration.validate(phase.name, position)
			if err != nil {
				return err
			}

			if previous, exists := seen[registration.name]; exists {
				return fmt.Errorf("%w: %q in %s and %s", ErrDuplicateName, registration.name, previous, phase.name)
			}

			seen[registration.name] = phase.name
		}
	}

	for _, phase := range phases {
		for _, registration := range phase.list {
			if validator, ok := registration.action.(Validator); ok {
				err := validator.Validate()
				if err != nil {
					return fmt.Errorf("validate %s action %q: %w", phase.name, registration.name, err)
				}
			}
		}
	}

	return nil
}

func (p *Pipeline) Run(ctx context.Context, request *Request) action.ActionError {
	err := p.Validate()
	if err != nil {
		return action.NewErrorW(err)
	}

	var outcome action.ActionError

	beforeOutcome := runPhase(ctx, request, p.options.Before, false, outcome)
	if beforeOutcome.IsTerminal() && beforeOutcome.Err() != nil {
		return runPhase(ctx, request, p.options.After, false, beforeOutcome)
	}

	mainOutcome := runPhase(ctx, request, p.options.Main, true, beforeOutcome)

	return runPhase(ctx, request, p.options.After, false, mainOutcome)
}

func (p *Pipeline) Cleanup(ctx context.Context, request *Request) action.ActionError {
	err := p.Validate()
	if err != nil {
		return action.NewErrorW(err)
	}

	return runPhase(ctx, request, p.options.Cleanup, true, action.ActionError{})
}

func (p *Pipeline) withRegistration(add func(*Options, Registration), registration Registration) *Pipeline {
	configured := Options{
		Before:  slices.Clone(p.options.Before),
		Main:    slices.Clone(p.options.Main),
		After:   slices.Clone(p.options.After),
		Cleanup: slices.Clone(p.options.Cleanup),
	}
	add(&configured, registration)

	return New(configured)
}
