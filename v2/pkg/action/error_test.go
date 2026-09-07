package action_test

import (
	"errors"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
)

var errPlain = errors.New("plain failure")

func TestActionErrorClassifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		build    func(action.ActionError) action.ActionError
		name     string
		wantType action.ErrorType
	}{
		{
			name: "terminal", build: func(value action.ActionError) action.ActionError { return value.Terminal() },
			wantType: action.ErrorTypeTerminal,
		},
		{
			name: "non blocking", build: func(value action.ActionError) action.ActionError { return value.NonBlocking() },
			wantType: action.ErrorTypeNonBlocking,
		},
		{
			name: "advisory", build: func(value action.ActionError) action.ActionError { return value.Advisory() },
			wantType: action.ErrorTypeAdvisory,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			value := test.build(action.NewErrorW(errPlain)).WithRequeueAfter(time.Second)
			g.Expect(value.Type()).Should(Equal(test.wantType))
			g.Expect(value.RequeueAfter()).Should(Equal(time.Second))
			g.Expect(value).Should(MatchError(errPlain.Error()))
			g.Expect(errors.Is(value, errPlain)).Should(BeTrue())
		})
	}
}

func TestActionErrorAddClassifiesAndAggregatesErrorTrees(t *testing.T) {
	t.Parallel()

	tests := []struct {
		returned  error
		name      string
		wantDelay time.Duration
		wantType  action.ErrorType
		wantStop  bool
	}{
		{name: "plain", returned: errPlain, wantType: action.ErrorTypeTerminal, wantStop: true},
		{name: "terminal", returned: action.NewError("terminal"), wantType: action.ErrorTypeTerminal, wantStop: true},
		{
			name: "non blocking", returned: action.NewError("non blocking").NonBlocking(),
			wantType: action.ErrorTypeNonBlocking,
		},
		{name: "advisory", returned: action.NewError("advisory").Advisory(), wantType: action.ErrorTypeAdvisory},
		{
			name: "advisory earliest delay",
			returned: errors.Join(
				action.NewError("late").Advisory().WithRequeueAfter(time.Minute),
				action.NewError("early").Advisory().WithRequeueAfter(time.Second),
			),
			wantType:  action.ErrorTypeAdvisory,
			wantDelay: time.Second,
		},
		{
			name: "plain overrides terminal delay in join",
			returned: errors.Join(
				action.NewError("delayed").WithRequeueAfter(time.Minute),
				fmt.Errorf("wrapped: %w", errPlain),
			),
			wantType:  action.ErrorTypeTerminal,
			wantStop:  true,
			wantDelay: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			g := NewWithT(t)
			result, stopped := (action.ActionError{}).Add("test", test.returned)
			g.Expect(result.Type()).Should(Equal(test.wantType))
			g.Expect(stopped).Should(Equal(test.wantStop))
			g.Expect(result.RequeueAfter()).Should(Equal(test.wantDelay))
			g.Expect(result).Should(MatchError(ContainSubstring("test")))
		})
	}
}
