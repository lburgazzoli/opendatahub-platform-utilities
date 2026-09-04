# V2 Action Error Semantics

This document is the authoritative error contract for the v2 action pipeline.
It defines how ordinary Go errors and semantic action errors control pipeline
execution, status interpretation, events, and reconciliation scheduling.

The v2 design keeps one error model. Actions return `error`; an
`action.ActionError` is used when the action intentionally communicates a
pipeline or reconciliation outcome. There is no second execution-report or
phase-specific error type.

## Scope

An action error has two independent concerns:

- whether later actions in the current phase may run; and
- whether the reconciliation is successful, failed, or advisory.

A plain Go error represents an unexpected failure. It is terminal, marks the
reconciliation as failed, and uses controller-runtime's normal error backoff.
An `ActionError` is appropriate when the action needs to select a semantic
outcome or an explicit retry delay.

The public constructors create terminal errors by default. Actions can change
the classifier with `Terminal`, `NonBlocking`, or `Advisory`, and can attach a
delay with `WithRequeueAfter`:

```go
return action.NewError("dependency unavailable").
	WithRequeueAfter(time.Minute)
```

The exact constructor set is part of `pkg/action`; the important contract is
that constructing an action error is side-effect free and that the error still
implements the standard Go `error` interface.

## Classifiers

`RequeueAfter` is independent metadata. Any classifier may carry a positive
explicit delay.

| Returned value | Outcome | Current-phase traversal | Reconcile result |
| --- | --- | --- | --- |
| `nil` | Successful | Continue | No explicit retry, so the reconciler may use its success default |
| Plain Go error | Failed | Stop the applicable phase | Return an error and use normal controller-runtime backoff |
| `Terminal` | Failed | Stop the applicable phase | Return an error, unless a positive delay is selected |
| `NonBlocking` | Failed | Continue | Return an error, unless a positive delay is selected |
| `Advisory` | Successful with context | Continue | Return success, optionally with a delayed requeue |

Terminal and non-blocking outcomes both make provisioning unsuccessful. Their
difference is traversal: a non-blocking outcome allows the remaining actions
in the applicable phase to execute. Advisory outcomes do not make
provisioning unsuccessful.

## Creating semantic errors

Use a plain error for an unexpected failure:

```go
return fmt.Errorf("reading Secret: %w", err)
```

Use a terminal error when the action cannot safely continue the main work and
knows when it should be tried again:

```go
return action.NewError("dependency unavailable").
	Terminal().
	WithRequeueAfter(time.Minute)
```

Use a non-blocking error when later actions can still contribute useful work:

```go
return action.NewError("optional resource failed").NonBlocking()
```

Use an advisory when the action wants to report context without failing the
reconciliation:

```go
return action.NewError("rollout still progressing").
	Advisory().
	WithRequeueAfter(30 * time.Second)
```

An action may wrap or join errors. Wrapping preserves the original cause for
`errors.Is` and `errors.As`. Joining retains all diagnostics; the accumulator
still determines the resulting classifier and delay according to the rules
below.

## Aggregation and precedence

The pipeline uses a single `ActionError` accumulator. It visits wrapped and
joined errors returned by each action and retains their diagnostics. The
pipeline does not create a second result model or perform a separate error
aggregation pass.

The precedence rules are:

1. A plain Go error or terminal action error makes the aggregate terminal. A
   plain Go error anywhere in the same joined error tree takes precedence over
   explicit delays and uses normal controller-runtime error backoff.
2. Otherwise, non-blocking outcomes outrank advisory outcomes and make the
   aggregate unsuccessful.
3. Advisory-only outcomes keep the aggregate successful while retaining their
   messages as context.
4. When semantic outcomes request delays, the earliest positive delay is
   selected. A delay of zero means that the outcome did not request an
   explicit delayed retry.

If an entire joined failure should use one explicit delay, classify the join
itself:

```go
return action.NewErrorW(errors.Join(secretErr, dependencyErr)).
	WithRequeueAfter(time.Minute)
```

## Phase traversal

The pipeline has four ordered phases: `before`, `main`, `after`, and
`cleanup`. Registration order is execution order. Guards are evaluated for an
action at reconciliation time; a false guard skips that action and produces
no error outcome.

Before and after actions use the same action, naming, guard, and error
contracts as main actions. They are not a separate kind of error.

Normal reconciliation behaves as follows:

1. Execute every eligible before action. A terminal or plain error prevents
   the main phase, but does not prevent later eligible before actions.
2. If the before aggregate is not blocking, execute main actions in
   registration order. A terminal or plain error stops the remaining main
   actions; non-blocking and advisory errors allow them to continue.
3. Execute every eligible after action, even when the main phase was skipped or
   stopped. A terminal or plain error is retained in the aggregate but does
   not prevent later eligible after actions.
4. The reconciler evaluates the complete aggregate, computes any opted-in
   status capabilities, applies status, and interprets the final result.

This means an after action can observe the resource snapshot and register
dynamic ownership watches independently of whether deployment succeeded. A
failure from that action still contributes to the final outcome.

Cleanup is invoked separately for deletion and is not part of normal apply
execution. Cleanup actions run in registration order. A plain or terminal
cleanup error stops later cleanup actions; non-blocking and advisory cleanup
errors continue. The finalizer is removed only after cleanup completes
successfully. A cleanup delay or error retains the finalizer so the controller
can retry.

## Status and conditions

The reconciler, not an action, owns final status processing. It consumes the
complete before/main/after aggregate after traversal has finished.

The common status contract contains observed generation and conditions. Release
status, phase status, and condition access are optional accessor capabilities;
the reconciler updates only capabilities implemented by the platform object.
Actions that report concrete conditions update the object's conditions through
its `ConditionsAccessor`; they do not put a condition manager or condition
definitions into the pipeline request.

The aggregate maps as follows:

- terminal and plain-error outcomes mark provisioning unsuccessful;
- non-blocking outcomes mark provisioning unsuccessful even though the rest of
  the applicable phase ran; and
- advisory-only outcomes leave provisioning successful and retain advisory
  context.

Status is applied after the pipeline, using the framework's server-side apply
status operation. A status-apply failure is an ordinary reconcile error and is
returned through the normal controller-runtime retry path. It does not replace
the action error model or trigger special pipeline handling.

The reconciler applies status before returning a delayed or error result. When
an explicit delay is selected, it returns a successful controller-runtime
result with that delay because controller-runtime ignores `RequeueAfter` when
`Reconcile` also returns a non-nil error:

```go
return ctrl.Result{RequeueAfter: delay}, nil
```

The controller's default success requeue applies only when the final outcome
is successful and no action requested a specific delay. It is not used during
deletion.

## Events and returned errors

The reconciler is responsible for translating the aggregate into controller
events and the final `ctrl.Result`; actions remain usable outside a
controller-runtime reconciler and do not need to know about scheduling.

- terminal and plain-error outcomes produce a warning event and a returned
  error unless an explicit positive delay was selected;
- non-blocking outcomes produce a warning event and follow the same returned
  error or explicit-delay rule; and
- advisory-only outcomes are successful and may schedule a delayed requeue.

The returned error retains the action names and joined diagnostics. Callers
may inspect it with `errors.Is` or `errors.As`.

## Migration from the v1 framework

The v2 package is `pkg/action`; the v1
`framework/controller/actions/errors` package is not part of the v2 public
layout. Deprecated `StopError` and `RequeueAfterError` markers are not carried
forward as v2 APIs.

Use these replacements:

- `StopError` becomes a terminal `ActionError`, with an explicit
  `WithRequeueAfter` when a fixed delay is required.
- `RequeueAfterError` becomes an advisory `ActionError` when it is informational
  and should not fail provisioning, or a non-blocking `ActionError` when the
  action must report failure while allowing later actions to run.
- A normal unexpected failure remains a plain Go error.

The v2 phase rules change only when later actions run. They do not change the
classifier precedence, diagnostic retention, status mapping, or
controller-runtime scheduling rules described above.
