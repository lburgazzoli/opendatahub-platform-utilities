// Package action contains domain-level action outcome semantics.
package action

import (
	"errors"
	"fmt"
	"time"
)

// ErrorType classifies an action outcome for pipeline traversal and status.
type ErrorType uint8

const (
	ErrorTypeTerminal ErrorType = iota + 1
	ErrorTypeNonBlocking
	ErrorTypeAdvisory
)

// ActionError is the one semantic error type used by v2 actions.
type ActionError struct {
	reason       error
	errorType    ErrorType
	requeueAfter time.Duration
}

func NewError(message string) ActionError {
	return NewErrorW(errors.New(message)) //nolint:err113 // The constructor owns the caller-provided action message.
}

func NewErrorf(format string, args ...any) ActionError {
	return NewErrorW(fmt.Errorf(format, args...)) //nolint:err113 // The constructor owns the caller-provided format.
}

func NewErrorW(reason error) ActionError {
	return ActionError{reason: reason, errorType: ErrorTypeTerminal}
}

func (e ActionError) Error() string {
	if e.reason == nil {
		return ""
	}

	return e.reason.Error()
}

func (e ActionError) Unwrap() error { return e.reason }

func (e ActionError) Err() error {
	if e.reason == nil {
		return nil
	}

	return e
}

func (e ActionError) Type() ErrorType { return e.errorType }

func (e ActionError) IsTerminal() bool { return e.errorType == ErrorTypeTerminal }

func (e ActionError) Terminal() ActionError {
	e.errorType = ErrorTypeTerminal
	return e
}

func (e ActionError) NonBlocking() ActionError {
	e.errorType = ErrorTypeNonBlocking
	return e
}

func (e ActionError) Advisory() ActionError {
	e.errorType = ErrorTypeAdvisory
	return e
}

func (e ActionError) RequeueAfter() time.Duration { return e.requeueAfter }

func (e ActionError) WithRequeueAfter(delay time.Duration) ActionError {
	e.requeueAfter = delay
	return e
}

// Add incorporates one action return and reports whether the applicable phase
// should stop. It retains all diagnostics in one joined error tree.
func (e ActionError) Add(actionName string, returned error) (ActionError, bool) {
	if returned == nil {
		return e, false
	}

	state := aggregation{
		reason: e.reason,
		typeOf: e.errorType,
		delay:  e.requeueAfter,
	}
	visitError(returned, actionName, &state)

	if state.terminal != nil {
		if state.plainTerminal {
			state.terminalDelay = 0
		}

		return ActionError{
			reason:       errors.Join(state.reason, errors.Join(state.diagnostics...), state.terminal),
			errorType:    ErrorTypeTerminal,
			requeueAfter: state.terminalDelay,
		}, true
	}

	return ActionError{
		reason:       errors.Join(state.reason, errors.Join(state.diagnostics...)),
		errorType:    moreSevere(state.typeOf, state.nonTerminalType),
		requeueAfter: earliestDelay(state.delay, state.requestedDelay),
	}, false
}
