package action

import (
	"fmt"
	"time"
)

type aggregation struct {
	reason           error
	terminal         error
	diagnostics      []error
	terminalDelay    time.Duration
	delay            time.Duration
	requestedDelay   time.Duration
	typeOf           ErrorType
	nonTerminalType  ErrorType
	terminalExplicit bool
	plainTerminal    bool
	plainError       bool
}

func visitError(value error, reported error, actionName string, state *aggregation) {
	if value == nil {
		return
	}

	if semantic, ok := asActionError(value); ok {
		handleSemanticError(semantic, actionName, reported, state)

		return
	}

	switch unwrapped := value.(type) { //nolint:errorlint // Visit one error-tree node at a time.
	case interface{ Unwrap() []error }:
		for _, cause := range unwrapped.Unwrap() {
			visitError(cause, reported, actionName, state)
		}
	case interface{ Unwrap() error }:
		visitError(unwrapped.Unwrap(), reported, actionName, state)
	default:
		setTerminal(fmt.Errorf("action %s: %w", actionName, reported), 0, false, state)
	}
}

func handleSemanticError(semantic ActionError, actionName string, value error, state *aggregation) {
	reported := fmt.Errorf("action %s: %w", actionName, value)

	switch semantic.Type() {
	case ErrorTypeTerminal:
		setTerminal(reported, semantic.RequeueAfter(), true, state)
	case ErrorTypeNonBlocking:
		state.diagnostics = append(state.diagnostics, reported)
		state.nonTerminalType = moreSevere(state.nonTerminalType, ErrorTypeNonBlocking)
		state.requestedDelay = earliestDelay(state.requestedDelay, semantic.RequeueAfter())
	case ErrorTypeAdvisory:
		state.diagnostics = append(state.diagnostics, reported)
		state.nonTerminalType = moreSevere(state.nonTerminalType, ErrorTypeAdvisory)
		state.requestedDelay = earliestDelay(state.requestedDelay, semantic.RequeueAfter())
	default:
		//nolint:err113 // Diagnostic wraps a fixed format.
		invalid := fmt.Errorf("action %s: invalid ActionError type %d", actionName, semantic.Type())
		setTerminal(invalid, 0, true, state)
	}
}

func setTerminal(reason error, delay time.Duration, explicit bool, state *aggregation) {
	if !explicit {
		state.plainTerminal = true
		state.plainError = true
	}

	switch {
	case state.terminal == nil:
		state.terminal = reason
		state.terminalDelay = delay
		state.terminalExplicit = explicit
	case explicit && !state.terminalExplicit:
		state.diagnostics = append(state.diagnostics, state.terminal)
		state.terminal = reason
		state.terminalDelay = delay
		state.terminalExplicit = true
	case explicit:
		state.diagnostics = append(state.diagnostics, reason)
		state.terminalDelay = earliestDelay(state.terminalDelay, delay)
	default:
		state.diagnostics = append(state.diagnostics, reason)
	}
}

func asActionError(value error) (ActionError, bool) {
	switch typed := value.(type) { //nolint:errorlint // The visitor handles the tree explicitly.
	case ActionError:
		return typed, true
	case *ActionError:
		if typed != nil {
			return *typed, true
		}
	}

	return ActionError{}, false
}

func earliestDelay(current, requested time.Duration) time.Duration {
	switch {
	case requested <= 0:
		return current
	case current <= 0:
		return requested
	default:
		return min(current, requested)
	}
}

func moreSevere(current, candidate ErrorType) ErrorType {
	if severity(candidate) > severity(current) {
		return candidate
	}

	return current
}

func severity(value ErrorType) int {
	switch value {
	case ErrorTypeTerminal:
		return 3
	case ErrorTypeNonBlocking:
		return 2
	case ErrorTypeAdvisory:
		return 1
	default:
		return 0
	}
}
