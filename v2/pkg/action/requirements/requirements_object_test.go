package requirements_test

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/requirements"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

func TestRequireObjectsChecksExactAndGVKTargets(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	configMapGVK := corev1.SchemeGroupVersion.WithKind("ConfigMap")
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "required",
			Namespace: "opendatahub",
		},
	}
	cli := testClient(configMap)
	conditions := &conditionAccessor{}

	exact := requirements.RequireObjects(requirements.ObjectReference{
		GVK:       configMapGVK,
		Namespace: configMap.Namespace,
		Name:      configMap.Name,
	})
	g.Expect(exact.Run(
		t.Context(),
		requirements.WithClient(cli),
		requirements.WithConditions(conditions),
		requirements.WithObservedGeneration(7),
	)).ShouldNot(HaveOccurred())

	available := condition.Find(conditions, requirements.ConditionTypeObjectsAvailable)
	g.Expect(available).ShouldNot(BeNil())
	g.Expect(available.Status).Should(Equal(metav1.ConditionTrue))
	g.Expect(available.ObservedGeneration).Should(Equal(int64(7)))

	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster-target"},
	}
	clusterScoped := requirements.RequireObjects(requirements.ObjectReference{
		GVK:  corev1.SchemeGroupVersion.WithKind("Namespace"),
		Name: namespace.Name,
	})
	g.Expect(clusterScoped.Run(
		t.Context(),
		requirements.WithClient(testClient(namespace)),
		requirements.WithConditions(&conditionAccessor{}),
	)).ShouldNot(HaveOccurred())

	anyConfigMap := requirements.RequireObjects(requirements.ObjectReference{
		GVK:       configMapGVK,
		Namespace: configMap.Namespace,
	})
	g.Expect(anyConfigMap.Execute(t.Context(), &pipeline.Request{
		Client:   cli,
		Instance: &platformObject{},
	})).ShouldNot(HaveOccurred())
}

func TestRequireObjectsRejectsMissingObjects(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	target := requirements.ObjectReference{
		GVK:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		Namespace: "opendatahub",
		Name:      "missing",
	}
	conditions := &conditionAccessor{}

	err := requirements.RequireObjects(target).Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(conditions),
		requirements.WithObservedGeneration(9),
	)

	g.Expect(err).Should(MatchError(ContainSubstring("require-objects")))
	actionErr, ok := errors.AsType[action.ActionError](err)
	g.Expect(ok).Should(BeTrue())
	g.Expect(actionErr.IsTerminal()).Should(BeTrue())

	missing := condition.Find(conditions, requirements.ConditionTypeObjectsAvailable)
	g.Expect(missing).ShouldNot(BeNil())
	g.Expect(missing.Status).Should(Equal(metav1.ConditionFalse))
	g.Expect(missing.Reason).Should(Equal("ObjectsUnavailable"))
	g.Expect(missing.Message).Should(ContainSubstring("v1, Kind=ConfigMap"))
}

func TestForbidObjectsChecksExactAndGVKTargets(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	configMapGVK := corev1.SchemeGroupVersion.WithKind("ConfigMap")
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "forbidden",
			Namespace: "opendatahub",
		},
	}
	conditions := &conditionAccessor{}

	exact := requirements.ForbidObjects(requirements.ObjectReference{
		GVK:       configMapGVK,
		Namespace: configMap.Namespace,
		Name:      configMap.Name,
	})
	err := exact.Run(
		t.Context(),
		requirements.WithClient(testClient(configMap)),
		requirements.WithConditions(conditions),
	)
	g.Expect(err).Should(MatchError(ContainSubstring("forbid-objects")))

	forbidden := condition.Find(conditions, requirements.ConditionTypeForbiddenObjectsAbsent)
	g.Expect(forbidden).ShouldNot(BeNil())
	g.Expect(forbidden.Status).Should(Equal(metav1.ConditionFalse))
	g.Expect(forbidden.Reason).Should(Equal("ForbiddenObjectsPresent"))

	g.Expect(exact.Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(&conditionAccessor{}),
	)).ShouldNot(HaveOccurred())

	anyConfigMap := requirements.ForbidObjects(requirements.ObjectReference{
		GVK:       configMapGVK,
		Namespace: configMap.Namespace,
	})
	g.Expect(anyConfigMap.Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(&conditionAccessor{}),
	)).ShouldNot(HaveOccurred())
}

func TestObjectRequirementsTreatMissingAPIsAsAbsence(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	target := requirements.ObjectReference{
		GVK:  schema.GroupVersionKind{Group: "example.io", Version: "v1", Kind: "Widget"},
		Name: "default",
	}

	forbiddenConditions := &conditionAccessor{}
	g.Expect(requirements.ForbidObjects(target).Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(forbiddenConditions),
	)).ShouldNot(HaveOccurred())
	g.Expect(condition.IsTrue(
		forbiddenConditions,
		requirements.ConditionTypeForbiddenObjectsAbsent,
	)).Should(BeTrue())

	requiredConditions := &conditionAccessor{}
	err := requirements.RequireObjects(target).Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(requiredConditions),
	)
	g.Expect(err).Should(HaveOccurred())
	g.Expect(condition.IsFalse(
		requiredConditions,
		requirements.ConditionTypeObjectsAvailable,
	)).Should(BeTrue())

	anyTarget := target
	anyTarget.Name = ""
	g.Expect(requirements.ForbidObjects(anyTarget).Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(&conditionAccessor{}),
	)).ShouldNot(HaveOccurred())

	err = requirements.RequireObjects(anyTarget).Run(
		t.Context(),
		requirements.WithClient(testClient()),
		requirements.WithConditions(&conditionAccessor{}),
	)
	g.Expect(err).Should(HaveOccurred())
}

func TestObjectRequirementsPropagateLookupErrors(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	conditions := &conditionAccessor{}
	failingGet := &failingClient{
		Client: testClient(),
		getErr: errors.New("get failed"),
	}

	err := requirements.RequireObjects(requirements.ObjectReference{
		GVK:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		Namespace: "opendatahub",
		Name:      "example",
	}).Run(
		t.Context(),
		requirements.WithClient(failingGet),
		requirements.WithConditions(conditions),
	)
	g.Expect(err).Should(MatchError(ContainSubstring("get failed")))
	g.Expect(condition.Find(conditions, requirements.ConditionTypeObjectsAvailable).Status).
		Should(Equal(metav1.ConditionUnknown))

	failingList := &failingClient{
		Client:  testClient(),
		listErr: errors.New("list failed"),
	}
	err = requirements.ForbidObjects(requirements.ObjectReference{
		GVK:       corev1.SchemeGroupVersion.WithKind("ConfigMap"),
		Namespace: "opendatahub",
	}).Run(
		t.Context(),
		requirements.WithClient(failingList),
		requirements.WithConditions(&conditionAccessor{}),
	)
	g.Expect(err).Should(MatchError(ContainSubstring("list failed")))
}

func TestObjectRequirementsValidateAndExecuteInputs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	var required requirements.RequireObjectsAction
	var forbidden requirements.ForbidObjectsAction

	g.Expect(required.Validate()).Should(MatchError(requirements.ErrActionRequired))
	g.Expect(forbidden.Validate()).Should(MatchError(requirements.ErrActionRequired))
	g.Expect(required.Name()).Should(Equal("require-objects"))
	g.Expect(forbidden.Name()).Should(Equal("forbid-objects"))

	g.Expect(requirements.RequireObjects().Validate()).Should(MatchError(requirements.ErrObjectsRequired))
	g.Expect(requirements.RequireObjects(requirements.ObjectReference{
		GVK: schema.GroupVersionKind{Kind: "ConfigMap"},
	}).Validate()).Should(MatchError(ContainSubstring("object GVK is required")))

	action := requirements.RequireObjects(requirements.ObjectReference{
		GVK: corev1.SchemeGroupVersion.WithKind("ConfigMap"),
	})
	g.Expect(action.Execute(t.Context(), nil)).Should(MatchError(requirements.ErrRequestRequired))
	g.Expect(action.Execute(t.Context(), &pipeline.Request{
		Client: testClient(),
	})).Should(MatchError(requirements.ErrInstanceRequired))
}

type failingClient struct {
	client.Client
	getErr  error
	listErr error
}

func (c *failingClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return c.getErr
}

func (c *failingClient) List(_ context.Context, _ client.ObjectList, _ ...client.ListOption) error {
	return c.listErr
}
