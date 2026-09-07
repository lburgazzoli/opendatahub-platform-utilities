package pipeline

import "context"

type Guard interface {
	Name() string
	Evaluate(ctx context.Context, request *Request) (bool, error)
}

type GuardFunc struct {
	EvaluateFunc func(context.Context, *Request) (bool, error)
	GuardName    string
}

func (g GuardFunc) Name() string { return g.GuardName }

func (g GuardFunc) Evaluate(ctx context.Context, request *Request) (bool, error) {
	if g.EvaluateFunc == nil {
		return true, nil
	}

	return g.EvaluateFunc(ctx, request)
}

func NewGuard(name string, evaluate func(context.Context, *Request) (bool, error)) Guard {
	return GuardFunc{GuardName: name, EvaluateFunc: evaluate}
}
