package pipeline

import (
	"errors"
	"fmt"
)

var (
	ErrNilAction          = errors.New("pipeline action is required")
	ErrNilGuard           = errors.New("pipeline guard is required")
	ErrActionNameRequired = errors.New("pipeline action name is required")
	ErrDuplicateName      = errors.New("pipeline action name is duplicated")
)

type Registration struct {
	action Action
	name   string
	guards []Guard
}

func Register(action Action, options ...ActionOption) Registration {
	configured := ActionOptions{}
	if action != nil {
		configured.Name = action.Name()
	}

	for _, option := range options {
		if option != nil {
			option.ApplyTo(&configured)
		}
	}

	return Registration{action: action, name: configured.Name, guards: append([]Guard(nil), configured.Guards...)}
}

func (r Registration) validate(phase string, position int) error {
	if r.action == nil {
		return fmt.Errorf("%s action %d: %w", phase, position, ErrNilAction)
	}

	if r.name == "" {
		return fmt.Errorf("%s action %d: %w", phase, position, ErrActionNameRequired)
	}

	for index, guard := range r.guards {
		if guard == nil {
			return fmt.Errorf("%s action %q guard %d: %w", phase, r.name, index, ErrNilGuard)
		}
	}

	return nil
}
