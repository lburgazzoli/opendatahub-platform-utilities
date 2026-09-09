package pipeline_test

import (
	"context"
	"errors"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

var errPipelineFailure = errors.New("pipeline failure")

type mockAction struct {
	mock.Mock

	name string
}

func (a *mockAction) Name() string { return a.name }

func (a *mockAction) Execute(ctx context.Context, request *pipeline.Request) error {
	return a.Called(ctx, request).Error(0)
}

type validatingAction struct {
	action           *mockAction
	validationCalled *bool
}

type countingValidator struct {
	validationCalls *int
}

func (a *countingValidator) Name() string { return "counted" }

func (a *countingValidator) Execute(context.Context, *pipeline.Request) error {
	return nil
}

func (a *countingValidator) Validate() error {
	*a.validationCalls++
	return nil
}

func (a *validatingAction) Name() string { return a.action.Name() }

func (a *validatingAction) Execute(ctx context.Context, request *pipeline.Request) error {
	return a.action.Execute(ctx, request)
}

func (a *validatingAction) Validate() error {
	*a.validationCalled = true
	return nil
}

func TestRunExecutesPhasesWithTheDocumentedContinuationRules(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	order := make([]string, 0, 4)
	beforeFailure := newMockAction("before-failure", &order, errPipelineFailure)
	beforeLater := newMockAction("before-later", &order, nil)
	main := newMockAction("main", &order, nil)
	afterFirst := newMockAction("after-first", &order, nil)
	afterSecond := newMockAction("after-second", &order, nil)

	main.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)

	result := pipeline.New().
		WithBeforeAction(beforeFailure).
		WithBeforeAction(beforeLater).
		WithAction(main).
		WithAfterAction(afterFirst).
		WithAfterAction(afterSecond).
		Run(t.Context(), &pipeline.Request{})

	g.Expect(result.Type()).Should(Equal(action.ErrorTypeTerminal))
	g.Expect(order).Should(Equal([]string{"before-failure", "before-later", "after-first", "after-second"}))
	beforeFailure.AssertExpectations(t)
	beforeLater.AssertExpectations(t)
	afterFirst.AssertExpectations(t)
	afterSecond.AssertExpectations(t)
}

func TestRunContinuesNonBlockingMainAndStopsAfterTerminalMain(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	order := make([]string, 0, 3)
	first := newMockAction("first", &order, action.NewError("optional").NonBlocking())
	second := newMockAction("second", &order, action.NewError("terminal"))
	third := newMockAction("third", &order, nil)
	after := newMockAction("after", &order, nil)

	result := pipeline.New().
		WithAction(first).
		WithAction(second).
		WithAction(third).
		WithAfterAction(after).
		Run(t.Context(), &pipeline.Request{})

	g.Expect(result.Type()).Should(Equal(action.ErrorTypeTerminal))
	g.Expect(order).Should(Equal([]string{"first", "second", "after"}))
	first.AssertExpectations(t)
	second.AssertExpectations(t)
	after.AssertExpectations(t)
	third.AssertNotCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestGuardsSkipWithoutOutcomeAndGuardErrorsBlock(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	called := false
	skipped := pipeline.ActionFunc{ActionName: "skipped", ExecuteFunc: func(_ context.Context, _ *pipeline.Request) error {
		called = true
		return nil
	}}
	guardError := pipeline.NewGuard("broken", func(context.Context, *pipeline.Request) (bool, error) {
		return false, errPipelineFailure
	})

	disabled := pipeline.NewGuard("disabled", func(context.Context, *pipeline.Request) (bool, error) {
		return false, nil
	})
	guarded := pipeline.Wrap(func(context.Context, *pipeline.Request) error { return nil })
	result := pipeline.New().
		WithAction(skipped, pipeline.When(disabled)).
		WithAction(
			guarded,
			pipeline.WithName("guard-error"),
			pipeline.When(guardError),
		).
		Run(t.Context(), &pipeline.Request{})

	g.Expect(called).Should(BeFalse())
	g.Expect(result.Err()).Should(MatchError(ContainSubstring("broken")))
}

func TestValidateRejectsNamesBeforeCallingValidators(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	validationCalled := false
	first := &validatingAction{
		action:           &mockAction{name: "duplicate"},
		validationCalled: &validationCalled,
	}
	second := &mockAction{name: "duplicate"}
	pipelineValue := pipeline.New().WithAction(first).WithAfterAction(second)

	g.Expect(pipelineValue.Validate()).Should(MatchError(ContainSubstring("duplicated")))
	g.Expect(validationCalled).Should(BeFalse())
}

func TestPipelineCachesValidation(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	validationCalls := 0
	pipelineValue := pipeline.New().WithAction(&countingValidator{
		validationCalls: &validationCalls,
	})

	g.Expect(pipelineValue.Validate()).Should(Succeed())
	g.Expect(pipelineValue.Run(t.Context(), &pipeline.Request{}).Err()).ShouldNot(HaveOccurred())
	g.Expect(pipelineValue.Cleanup(t.Context(), &pipeline.Request{}).Err()).ShouldNot(HaveOccurred())
	g.Expect(validationCalls).Should(Equal(1))
}

func TestHasCleanupActions(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	pipelineValue := pipeline.New()

	g.Expect(pipelineValue.HasCleanupActions()).Should(BeFalse())

	pipelineValue = pipelineValue.WithCleanupAction(pipeline.ActionFunc{
		ActionName: "cleanup",
	})

	g.Expect(pipelineValue.HasCleanupActions()).Should(BeTrue())
}

func TestCleanupStopsOnlyOnBlockingErrors(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	order := make([]string, 0, 2)
	first := newMockAction("cleanup-first", &order, action.NewError("cleanup").NonBlocking())
	second := newMockAction("cleanup-second", &order, nil)

	result := pipeline.New().WithCleanupAction(first).WithCleanupAction(second).Cleanup(t.Context(), &pipeline.Request{})

	g.Expect(result.Type()).Should(Equal(action.ErrorTypeNonBlocking))
	g.Expect(order).Should(Equal([]string{"cleanup-first", "cleanup-second"}))
	first.AssertExpectations(t)
	second.AssertExpectations(t)
}

func newMockAction(name string, order *[]string, returned error) *mockAction {
	actionValue := &mockAction{name: name}
	actionValue.On("Execute", mock.Anything, mock.Anything).Run(func(mock.Arguments) {
		*order = append(*order, name)
	}).Return(returned).Once()

	return actionValue
}
