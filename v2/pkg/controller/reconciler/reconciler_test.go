package reconciler

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestReconcileLoadsFreshObjectAndProjectsStatus(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	object := testObjectInstance("component")
	kubernetesClient := testClient(object)
	profile := api.PlatformProfile{Kind: "OpenShift", Version: "4.18"}
	var processed *testObject
	var extensions pipeline.Extension

	actionValue := pipeline.ActionFunc{
		ActionName: "observe",
		ExecuteFunc: func(_ context.Context, request *pipeline.Request) error {
			extensions = request.Extensions
			current, ok := request.Instance.(*testObject)
			if !ok {
				return errors.New("test action received an unexpected object") //nolint:err113 // Test-only type guard.
			}

			processed = current
			conditions, ok := request.Instance.(api.ConditionsAccessor)
			if !ok {
				return errors.New("conditions accessor is required") //nolint:err113 // Test-only setup failure.
			}

			condition.MarkTrue(conditions, "DependenciesAvailable")
			return nil
		},
	}
	reconcilerValue := &Reconciler{
		client:    kubernetesClient,
		scheme:    testScheme(),
		prototype: testObjectInstance("prototype"),
		pipeline:  pipeline.New().WithAction(actionValue),
		options: Options{
			ControllerName:  "component",
			FieldOwner:      "component",
			PlatformProfile: &profile,
		},
		recorder: record.NewFakeRecorder(10),
	}

	result, err := reconcilerValue.Reconcile(t.Context(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(result).Should(BeZero())
	g.Expect(processed).ShouldNot(BeNil())
	g.Expect(extensions).Should(Equal(pipeline.Extension{
		pipeline.ExtensionControllerName: "component",
		pipeline.ExtensionFieldOwner:     "component",
	}))
	g.Expect(processed.Status.ObservedGeneration).Should(Equal(processed.GetGeneration()))
	g.Expect(processed.Status.Platform).Should(Equal(&profile))
	g.Expect(processed.Status.Conditions).Should(ContainElement(HaveField("Type", "ProvisioningSucceeded")))
	g.Expect(processed.Status.Phase).Should(Equal(api.PhaseReady))

	updated := &testObject{}
	updated.SetGroupVersionKind(testObjectGVK)
	g.Expect(kubernetesClient.Get(t.Context(), client.ObjectKeyFromObject(object), updated)).Should(Succeed())
}

func TestReconcileReturnsSemanticOutcomeAfterStatusApply(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		returned error
		wantErr  bool
		wantWait time.Duration
	}{
		"plain error": {
			returned: errors.New("deployment failed"), //nolint:err113 // Test-only action outcome.
			wantErr:  true,
		},
		"advisory requeue": {
			returned: action.NewError("rollout progressing").Advisory().WithRequeueAfter(time.Minute),
			wantWait: time.Minute,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			object := testObjectInstance("component")
			reconcilerValue := &Reconciler{
				client:    testClient(object),
				scheme:    testScheme(),
				prototype: testObjectInstance("prototype"),
				pipeline: pipeline.New().WithAction(pipeline.ActionFunc{
					ActionName: "deploy",
					ExecuteFunc: func(context.Context, *pipeline.Request) error {
						return test.returned
					},
				}),
				options: Options{
					ControllerName: "component",
					FieldOwner:     "component",
				},
				recorder: record.NewFakeRecorder(10),
			}

			result, err := reconcilerValue.Reconcile(
				t.Context(),
				ctrl.Request{NamespacedName: client.ObjectKeyFromObject(object)},
			)

			if test.wantErr {
				g.Expect(err).Should(MatchError(ContainSubstring("deployment failed")))
			} else {
				g.Expect(err).ShouldNot(HaveOccurred())
			}

			g.Expect(result.RequeueAfter).Should(Equal(test.wantWait))
		})
	}
}
