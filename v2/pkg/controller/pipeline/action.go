// Package pipeline contains phased controller action registration and execution.
package pipeline

import (
	"context"
	"reflect"
	"runtime"
)

type Action interface {
	Name() string
	Execute(ctx context.Context, request *Request) error
}

type Validator interface{ Validate() error }

type ActionFunc struct {
	ExecuteFunc func(context.Context, *Request) error
	ActionName  string
}

func (a ActionFunc) Name() string { return a.ActionName }

func (a ActionFunc) Execute(ctx context.Context, request *Request) error {
	if a.ExecuteFunc == nil {
		return nil
	}

	return a.ExecuteFunc(ctx, request)
}

func Wrap(execute func(context.Context, *Request) error) Action {
	return ActionFunc{ActionName: functionName(execute), ExecuteFunc: execute}
}

func functionName(execute func(context.Context, *Request) error) string {
	if execute == nil {
		return ""
	}

	value := reflect.ValueOf(execute)

	function := runtime.FuncForPC(value.Pointer())
	if function == nil {
		return ""
	}

	return function.Name()
}
