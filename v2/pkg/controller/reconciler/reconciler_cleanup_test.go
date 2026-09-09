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
			return errors.New("must not run") //nolint:err113 // Test-only action outcome.
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

func deletingObject(deletionTime time.Time) *testObject {
	object := testObjectInstance("component")
	object.SetFinalizers([]string{DefaultFinalizerName})
	timestamp := metav1.NewTime(deletionTime)
	object.SetDeletionTimestamp(&timestamp)

	return object
}

func cleanupClient() client.Client {
	base := testClient(testObjectInstance("component"))
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
