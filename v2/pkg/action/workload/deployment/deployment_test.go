package deployment_test

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/workload/deployment"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

func TestRunObservesDeploymentAvailability(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	g.Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
	deploymentObject := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ready", Namespace: "ns", Labels: map[string]string{"managed": "true"}},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deploymentObject).Build()

	observation, err := deployment.New(
		deployment.WithSelectorLabels(map[string]string{"managed": "true"}),
	).Run(t.Context(), deployment.RunOptions{Client: cli, Namespace: "ns"})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(observation.Ready).Should(Equal(1))
	accessor := &conditionAccessor{conditions: []api.Condition{observation.Condition}}
	g.Expect(condition.IsTrue(accessor, observation.Condition.Type)).Should(BeTrue())
}

func TestNewRequiresSelector(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := deployment.New()

	g.Expect(action.Validate()).Should(MatchError(deployment.ErrSelectorRequired))
}

func TestOptionsTreatEmptyFunctionalValuesAsExplicit(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	labels := map[string]string{"managed": "true"}

	structAction := deployment.New(deployment.Options{
		Labels:             labels,
		ConditionType:      "",
		NotAvailableReason: deployment.DefaultNotAvailableReason,
	})
	functionalAction := deployment.New(
		deployment.WithSelectorLabels(labels),
		deployment.WithConditionType(""),
	)

	g.Expect(structAction.Validate()).Should(MatchError(deployment.ErrConditionTypeRequired))
	g.Expect(functionalAction.Validate()).Should(MatchError(deployment.ErrConditionTypeRequired))
}

func TestExecuteRejectsMissingRequest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := deployment.New(deployment.WithSelectorLabels(map[string]string{"managed": "true"}))

	err := action.Execute(t.Context(), nil)

	g.Expect(err).Should(MatchError(deployment.ErrRequestRequired))
}

func TestExecuteStoresDeploymentCondition(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	g.Expect(appsv1.AddToScheme(scheme)).Should(Succeed())
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "ready", Namespace: "ns", Labels: map[string]string{"managed": "true"}},
		Status:     appsv1.DeploymentStatus{Replicas: 1, ReadyReplicas: 1},
	}).Build()
	instance := &platformObject{ConfigMap: corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns"}}}

	action := deployment.New(deployment.WithSelectorLabels(map[string]string{"managed": "true"}))
	err := action.Execute(t.Context(), &pipeline.Request{
		Client:   cli,
		Instance: instance,
	})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(condition.IsTrue(instance.GetStatus(), deployment.DefaultConditionType)).Should(BeTrue())
}

type conditionAccessor struct{ conditions []api.Condition }

func (a *conditionAccessor) GetConditions() []api.Condition       { return a.conditions }
func (a *conditionAccessor) SetConditions(values []api.Condition) { a.conditions = values }

//nolint:govet // test fixture layout mirrors the embedded platform object.
type platformObject struct {
	corev1.ConfigMap

	status   api.Status
	releases api.ReleaseStatus
}

func (o *platformObject) GetStatus() *api.Status { return &o.status }

func (o *platformObject) GetReleaseStatus() *api.ReleaseStatus { return &o.releases }

func (o *platformObject) SetReleaseStatus(value api.ReleaseStatus) {
	o.releases = value
}

var _ api.PlatformObject = (*platformObject)(nil)
