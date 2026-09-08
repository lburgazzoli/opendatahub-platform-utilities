package reconciler

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileUsesConditionManagerFactory(t *testing.T) {
	t.Parallel()

	object := testObjectInstance("component")
	manager := &recordingConditionManager{}
	kubernetesClient := testClient(object)
	reconcilerValue := &Reconciler{
		client:    kubernetesClient,
		scheme:    testScheme(),
		prototype: testObjectInstance("prototype"),
		pipeline: pipeline.New().WithAction(pipeline.ActionFunc{
			ActionName: "observe",
			ExecuteFunc: func(_ context.Context, _ *pipeline.Request) error {
				return nil
			},
		}),
		options: Options{
			ControllerName: "component",
			FieldOwner:     "component",
			ConditionManager: func(_ api.ConditionsAccessor) ConditionManager {
				return manager
			},
		},
		recorder: record.NewFakeRecorder(10),
	}

	result, err := reconcilerValue.Reconcile(
		t.Context(),
		ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)},
	)

	g := NewWithT(t)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result).Should(BeZero())
	g.Expect(manager.called).Should(BeTrue())
	g.Expect(manager.generation).Should(Equal(object.GetGeneration()))
}

type recordingConditionManager struct {
	called     bool
	generation int64
}

func (m *recordingConditionManager) Apply(_ action.ActionError, generation int64) {
	m.called = true
	m.generation = generation
}
