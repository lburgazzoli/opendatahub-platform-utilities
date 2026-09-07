package condition

import (
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

// MarkOptions configures a condition mutation.
type MarkOptions struct {
	Reason             string
	Message            string
	Error              error
	ObservedGeneration *int64
	Severity           api.ConditionSeverity
}

// ApplyTo applies a complete option value to target.
func (o MarkOptions) ApplyTo(target *MarkOptions) {
	*target = o
	if o.ObservedGeneration != nil {
		generation := *o.ObservedGeneration
		target.ObservedGeneration = &generation
	}
}

// MarkOption configures a condition mutation.
type MarkOption = option.Option[MarkOptions]

// WithReason sets the condition reason.
func WithReason(reason string) MarkOption {
	return option.FunctionalOption[MarkOptions](func(options *MarkOptions) {
		options.Reason = reason
	})
}

// WithMessage formats and sets the condition message.
func WithMessage(format string, args ...any) MarkOption {
	return option.FunctionalOption[MarkOptions](func(options *MarkOptions) {
		options.Message = fmt.Sprintf(format, args...)
	})
}

// WithObservedGeneration records the generation that produced the condition.
func WithObservedGeneration(generation int64) MarkOption {
	return option.FunctionalOption[MarkOptions](func(options *MarkOptions) {
		options.ObservedGeneration = new(generation)
	})
}

// WithSeverity sets the condition severity.
func WithSeverity(severity api.ConditionSeverity) MarkOption {
	return option.FunctionalOption[MarkOptions](func(options *MarkOptions) {
		options.Severity = severity
	})
}

// WithError records an error as the condition reason and message.
func WithError(err error) MarkOption {
	return option.FunctionalOption[MarkOptions](func(options *MarkOptions) {
		options.Error = err
		options.Severity = api.ConditionSeverityError

		options.Reason = "Error"
		if err != nil {
			options.Message = err.Error()
		}
	})
}
