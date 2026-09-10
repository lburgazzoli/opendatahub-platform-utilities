package requirements_test

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/requirements"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

func TestRequireAPIsMarksAvailableAndUnavailable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	cli := testClient()
	conditions := &conditionAccessor{}

	action := requirements.RequireAPIs(
		corev1.SchemeGroupVersion.WithKind("ConfigMap"),
	)
	err := action.Run(
		t.Context(),
		requirements.WithClient(cli),
		requirements.WithConditions(conditions),
		requirements.WithObservedGeneration(4),
	)
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(condition.IsTrue(conditions, requirements.ConditionTypeDependenciesAvailable)).Should(BeTrue())

	action = requirements.RequireAPIs(
		corev1.SchemeGroupVersion.WithKind("Secret"),
	)
	err = action.Run(
		t.Context(),
		requirements.WithClient(cli),
		requirements.WithConditions(conditions),
	)
	g.Expect(err).Should(MatchError(ContainSubstring("require-apis")))
	requiredCondition := condition.Find(conditions, requirements.ConditionTypeDependenciesAvailable)
	g.Expect(requiredCondition).ShouldNot(BeNil())
	g.Expect(requiredCondition.Reason).Should(Equal("APIsUnavailable"))
}

func TestForbidAPIsRejectsAvailableAPI(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	conditions := &conditionAccessor{conditions: []api.Condition{{
		Type:   requirements.ConditionTypeDependenciesAvailable,
		Status: metav1.ConditionTrue,
	}}}
	action := requirements.ForbidAPIs(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	err := action.Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(conditions),
	)
	g.Expect(err).Should(MatchError(ContainSubstring("forbid-apis")))
	forbiddenCondition := condition.Find(conditions, requirements.ConditionTypeForbiddenAPIsAbsent)
	g.Expect(forbiddenCondition).ShouldNot(BeNil())
	g.Expect(forbiddenCondition.Reason).Should(Equal("ForbiddenAPIsAvailable"))
	g.Expect(condition.IsTrue(conditions, requirements.ConditionTypeDependenciesAvailable)).Should(BeTrue())
}

func TestRequirementRejectsMissingInputs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := requirements.RequireAPIs(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	err := action.Run(t.Context(), requirements.WithConditions(&conditionAccessor{}))
	g.Expect(err).Should(MatchError(ContainSubstring("client")))

	err = action.Run(t.Context(), requirements.WithClient(testClient()))
	g.Expect(err).Should(MatchError(ContainSubstring("conditions")))
}

func TestExecuteUsesInstanceConditions(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	instance := &platformObject{}
	action := requirements.RequireAPIs(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	err := action.Execute(t.Context(), &pipeline.Request{
		Client:   testClient(),
		Instance: instance,
	})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(condition.IsTrue(instance, requirements.ConditionTypeDependenciesAvailable)).Should(BeTrue())
}

func TestZeroValueActionsAreSafe(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	var required requirements.RequireAPIsAction
	var forbidden requirements.ForbidAPIsAction

	g.Expect(required.Validate()).Should(MatchError(requirements.ErrActionRequired))
	g.Expect(forbidden.Validate()).Should(MatchError(requirements.ErrActionRequired))
	g.Expect(required.Name()).Should(Equal("require-apis"))
	g.Expect(forbidden.Name()).Should(Equal("forbid-apis"))
}

func TestExecuteRejectsMissingRequestInputs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := requirements.RequireAPIs(corev1.SchemeGroupVersion.WithKind("ConfigMap"))

	err := action.Execute(t.Context(), nil)
	g.Expect(err).Should(MatchError(requirements.ErrRequestRequired))

	err = action.Execute(t.Context(), &pipeline.Request{Client: testClient()})
	g.Expect(err).Should(MatchError(requirements.ErrInstanceRequired))
}

type conditionAccessor struct {
	conditions []api.Condition
}

func (a *conditionAccessor) GetConditions() []api.Condition { return a.conditions }

func (a *conditionAccessor) SetConditions(values []api.Condition) {
	a.conditions = values
}

func testClient(objects ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("Namespace"), meta.RESTScopeRoot)

	return fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(restMapper).WithObjects(objects...).Build()
}

//nolint:govet // test fixture layout mirrors the embedded platform object.
type platformObject struct {
	corev1.ConfigMap

	status     api.Status
	conditions []api.Condition
	releases   api.ReleaseStatus
}

func (o *platformObject) GetStatus() *api.Status { return &o.status }

func (o *platformObject) GetReleaseStatus() *api.ReleaseStatus { return &o.releases }

func (o *platformObject) SetReleaseStatus(value api.ReleaseStatus) {
	o.releases = value
}

func (o *platformObject) GetConditions() []api.Condition { return o.conditions }

func (o *platformObject) SetConditions(values []api.Condition) {
	o.conditions = values
}

var _ api.PlatformObject = (*platformObject)(nil)
