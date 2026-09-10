package reconciler

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

var (
	errCleanupPlainFailure = errors.New("cleanup plain failure")       //nolint:gochecknoglobals,err113 // Shared test outcome.
	errCleanupUpdate       = errors.New("cleanup update failed")       //nolint:gochecknoglobals,err113 // Shared test outcome.
	errCleanupMustNotRun   = errors.New("cleanup action must not run") //nolint:gochecknoglobals,err113 // Shared test outcome.
)

func TestReconcileInstallsCleanupFinalizerBeforeNormalActions(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	object := testObjectInstance("component")
	kubernetesClient := testClient(object)
	finalizerName := "example.io/finalizer"
	called := false
	reconcilerValue := &Reconciler{
		client:    kubernetesClient,
		scheme:    testScheme(),
		prototype: testObjectInstance("prototype"),
		pipeline: pipeline.New().WithAction(pipeline.ActionFunc{
			ActionName: "normal",
			ExecuteFunc: func(context.Context, *pipeline.Request) error {
				called = true
				return nil
			},
		}).WithCleanupAction(pipeline.ActionFunc{
			ActionName: "cleanup",
		}),
		options: Options{
			ControllerName: "component",
			FieldOwner:     "component",
			FinalizerName:  finalizerName,
		},
		recorder: record.NewFakeRecorder(10),
	}

	_, err := reconcilerValue.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(called).Should(BeFalse())
	updated := &testObject{}
	updated.SetGroupVersionKind(testObjectGVK)
	g.Expect(kubernetesClient.Get(t.Context(), client.ObjectKeyFromObject(object), updated)).Should(Succeed())
	g.Expect(updated.GetFinalizers()).Should(ContainElement(finalizerName))
}

func TestReconcileRunsCleanupAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	kubernetesClient := cleanupClient()
	object := deletingObject(time.Now())
	called := false
	reconcilerValue := cleanupReconciler(kubernetesClient, pipeline.New().WithCleanupAction(pipeline.ActionFunc{
		ActionName: "cleanup",
		ExecuteFunc: func(context.Context, *pipeline.Request) error {
			called = true
			return nil
		},
	}))

	result, err := reconcilerValue.cleanup(t.Context(), object)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result).Should(BeZero())
	g.Expect(called).Should(BeTrue())
	g.Expect(object.GetFinalizers()).ShouldNot(ContainElement(DefaultFinalizerName))
}

func TestCleanupAdvisoryWithoutRequeueCompletes(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	kubernetesClient := cleanupClient()
	object := deletingObject(time.Now())
	reconcilerValue := cleanupReconciler(kubernetesClient, pipeline.New().WithCleanupAction(pipeline.ActionFunc{
		ActionName: "cleanup",
		ExecuteFunc: func(context.Context, *pipeline.Request) error {
			return action.NewError("already absent").Advisory()
		},
	}))

	_, err := reconcilerValue.cleanup(t.Context(), object)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(object.GetFinalizers()).ShouldNot(ContainElement(DefaultFinalizerName))
}

func TestCleanupDeadlineSkipsActionsAndRemovesFinalizer(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	kubernetesClient := cleanupClient()
	object := deletingObject(time.Now().Add(-time.Minute))
	called := false
	reconcilerValue := cleanupReconciler(kubernetesClient, pipeline.New().WithCleanupAction(pipeline.ActionFunc{
		ActionName: "cleanup",
		ExecuteFunc: func(context.Context, *pipeline.Request) error {
			called = true
			return errCleanupMustNotRun
		},
	}))
	reconcilerValue.options.CleanupTimeout = new(time.Minute)

	_, err := reconcilerValue.cleanup(t.Context(), object)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(called).Should(BeFalse())
	g.Expect(object.GetFinalizers()).ShouldNot(ContainElement(DefaultFinalizerName))
}

func TestFinishCleanupSkipsUpdateWhenFinalizerIsAbsent(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	object := testObjectInstance("component")
	baseClient := testClient(object)
	withWatch, ok := baseClient.(client.WithWatch)
	if !ok {
		t.Fatal("test client must implement client.WithWatch")
	}

	updates := 0
	kubernetesClient := interceptor.NewClient(withWatch, interceptor.Funcs{
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
			updates++

			return nil
		},
	})
	reconcilerValue := cleanupReconciler(kubernetesClient, pipeline.New())

	result, err := reconcilerValue.finishCleanup(
		t.Context(),
		object,
		corev1.EventTypeNormal,
		"cleanup completed",
	)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result).Should(BeZero())
	g.Expect(updates).Should(BeZero())
}

func TestCleanupRequeueEmitsEventWhenDelayIsCapped(t *testing.T) {
	t.Parallel()

	recorder := record.NewFakeRecorder(1)
	reconcilerValue := cleanupReconciler(cleanupClient(), pipeline.New())
	reconcilerValue.recorder = recorder

	deadline := time.Now().Add(time.Minute)
	outcome := action.NewError("cleanup still progressing").Advisory().WithRequeueAfter(time.Hour)

	result, err := reconcilerValue.cleanupRequeue(
		t.Context(),
		deletingObject(time.Now()),
		outcome,
		deadline,
		true,
	)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result.RequeueAfter).Should(BeNumerically("<", time.Hour))
	g.Expect(<-recorder.Events).Should(ContainSubstring("Normal Cleanup cleanup still progressing"))
}

func TestCleanupRetainsFinalizerForBlockingOutcomes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		returned error
	}{
		{name: "plain", returned: errCleanupPlainFailure},
		{name: "terminal", returned: action.NewError("terminal cleanup failure")},
		{name: "non-blocking", returned: action.NewError("optional cleanup failure").NonBlocking()},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorder := record.NewFakeRecorder(1)
			reconcilerValue := cleanupReconciler(
				cleanupClient(),
				pipeline.New().WithCleanupAction(pipeline.ActionFunc{
					ActionName: "cleanup",
					ExecuteFunc: func(context.Context, *pipeline.Request) error {
						return test.returned
					},
				}),
			)
			reconcilerValue.recorder = recorder
			object := deletingObject(time.Now())

			_, err := reconcilerValue.cleanup(t.Context(), object)

			g := NewWithT(t)
			g.Expect(err).Should(HaveOccurred())
			g.Expect(object.GetFinalizers()).Should(ContainElement(DefaultFinalizerName))
			g.Expect(<-recorder.Events).Should(ContainSubstring("Warning Cleanup"))
		})
	}
}

func TestCleanupDelayedOutcomesRetainFinalizerAndClassifyEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		returned  error
		eventType string
	}{
		{
			name:      "terminal",
			returned:  action.NewError("terminal cleanup failure").WithRequeueAfter(time.Second),
			eventType: corev1.EventTypeWarning,
		},
		{
			name:      "non-blocking",
			returned:  action.NewError("optional cleanup failure").NonBlocking().WithRequeueAfter(time.Second),
			eventType: corev1.EventTypeWarning,
		},
		{
			name:      "advisory",
			returned:  action.NewError("cleanup still progressing").Advisory().WithRequeueAfter(time.Second),
			eventType: corev1.EventTypeNormal,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			recorder := record.NewFakeRecorder(1)
			reconcilerValue := cleanupReconciler(
				cleanupClient(),
				pipeline.New().WithCleanupAction(pipeline.ActionFunc{
					ActionName: "cleanup",
					ExecuteFunc: func(context.Context, *pipeline.Request) error {
						return test.returned
					},
				}),
			)
			reconcilerValue.recorder = recorder
			object := deletingObject(time.Now())

			result, err := reconcilerValue.cleanup(t.Context(), object)

			g := NewWithT(t)
			g.Expect(err).ShouldNot(HaveOccurred())
			g.Expect(result.RequeueAfter).Should(Equal(time.Second))
			g.Expect(object.GetFinalizers()).Should(ContainElement(DefaultFinalizerName))
			g.Expect(<-recorder.Events).Should(ContainSubstring(test.eventType + " Cleanup"))
		})
	}
}

func TestCleanupDeadlineExpiryDuringActionRemovesFinalizer(t *testing.T) {
	t.Parallel()

	recorder := record.NewFakeRecorder(1)
	reconcilerValue := cleanupReconciler(
		cleanupClient(),
		pipeline.New().WithCleanupAction(pipeline.ActionFunc{
			ActionName: "cleanup",
			ExecuteFunc: func(ctx context.Context, _ *pipeline.Request) error {
				<-ctx.Done()

				return ctx.Err()
			},
		}),
	)
	reconcilerValue.recorder = recorder
	reconcilerValue.options.CleanupTimeout = new(10 * time.Millisecond)
	object := deletingObject(time.Now())

	result, err := reconcilerValue.cleanup(t.Context(), object)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result).Should(BeZero())
	g.Expect(object.GetFinalizers()).ShouldNot(ContainElement(DefaultFinalizerName))
	g.Expect(<-recorder.Events).Should(ContainSubstring("Warning Cleanup cleanup deadline reached"))
}

func TestFinishCleanupReturnsFinalizerUpdateError(t *testing.T) {
	t.Parallel()

	baseClient := testClient(testObjectInstance("component"))
	withWatch, ok := baseClient.(client.WithWatch)
	if !ok {
		t.Fatal("test client must implement client.WithWatch")
	}

	kubernetesClient := interceptor.NewClient(withWatch, interceptor.Funcs{
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
			return errCleanupUpdate
		},
	})
	recorder := record.NewFakeRecorder(1)
	reconcilerValue := cleanupReconciler(kubernetesClient, pipeline.New())
	reconcilerValue.recorder = recorder

	_, err := reconcilerValue.finishCleanup(
		t.Context(),
		deletingObject(time.Now()),
		corev1.EventTypeNormal,
		"cleanup completed",
	)

	g := NewWithT(t)
	g.Expect(err).Should(MatchError(ContainSubstring("remove reconciler finalizer")))
	g.Expect(recorder.Events).Should(BeEmpty())
}

func TestReconcileDeletionSkipsNormalActions(t *testing.T) {
	t.Parallel()

	object := deletingObject(time.Now())
	kubernetesClient := cleanupClientFor(object)
	normalCalled := false
	cleanupCalled := false
	reconcilerValue := cleanupReconciler(
		kubernetesClient,
		pipeline.New().
			WithAction(pipeline.ActionFunc{
				ActionName: "normal",
				ExecuteFunc: func(context.Context, *pipeline.Request) error {
					normalCalled = true
					return nil
				},
			}).
			WithCleanupAction(pipeline.ActionFunc{
				ActionName: "cleanup",
				ExecuteFunc: func(context.Context, *pipeline.Request) error {
					cleanupCalled = true
					return nil
				},
			}),
	)

	_, err := reconcilerValue.Reconcile(
		t.Context(),
		ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)},
	)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(normalCalled).Should(BeFalse())
	g.Expect(cleanupCalled).Should(BeTrue())
}

func deletingObject(deletionTime time.Time) *testObject {
	object := testObjectInstance("component")
	object.SetFinalizers([]string{DefaultFinalizerName})
	timestamp := metav1.NewTime(deletionTime)
	object.SetDeletionTimestamp(&timestamp)

	return object
}

func cleanupClient() client.Client {
	return cleanupClientFor(testObjectInstance("component"))
}

func cleanupClientFor(object client.Object) client.Client {
	base := testClient(object)
	withWatch, ok := base.(client.WithWatch)
	if !ok {
		panic("test client must implement client.WithWatch")
	}

	return interceptor.NewClient(withWatch, interceptor.Funcs{
		Update: func(context.Context, client.WithWatch, client.Object, ...client.UpdateOption) error {
			return nil
		},
	})
}

func cleanupReconciler(kubernetesClient client.Client, value *pipeline.Pipeline) *Reconciler {
	return &Reconciler{
		client:    kubernetesClient,
		scheme:    testScheme(),
		prototype: testObjectInstance("prototype"),
		pipeline:  value,
		options: Options{
			ControllerName: "component",
			FieldOwner:     "component",
			FinalizerName:  DefaultFinalizerName,
			CleanupTimeout: new(time.Duration(0)),
		},
		recorder: record.NewFakeRecorder(10),
	}
}
