package reconciler

import (
	"errors"
	"slices"
	"testing"

	. "github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestInstance(t *testing.T) {
	t.Parallel()

	instance := testObjectInstance("component")
	request := &pipeline.Request{Instance: instance}

	g := NewWithT(t)
	actual, err := Instance[*testObject](request)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(actual).Should(BeIdenticalTo(instance))
}

func TestInstanceRejectsUnexpectedType(t *testing.T) {
	t.Parallel()

	request := &pipeline.Request{Instance: testObjectInstance("component")}

	g := NewWithT(t)
	actual, err := Instance[*instanceTestObject](request)

	g.Expect(actual).Should(BeNil())
	g.Expect(err).Should(MatchError(MatchRegexp("reconciler request instance has unexpected type")))
}

func TestInstanceRejectsNilRequest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	actual, err := Instance[*testObject](nil)

	g.Expect(actual).Should(BeNil())
	g.Expect(err).Should(MatchError(MatchRegexp("reconciler request instance has unexpected type: request is nil")))
}

func TestConditionAggregationUsesConfiguredTypes(t *testing.T) {
	t.Parallel()

	object := testObjectInstance("component")
	object.Status.Conditions = []api.Condition{
		{
			Type:   string(api.ConditionTypeProvisioningSucceeded),
			Status: v1.ConditionTrue,
		},
		{
			Type:   string(api.ConditionTypeDegraded),
			Status: v1.ConditionFalse,
		},
		{
			Type:   "Unlisted",
			Status: v1.ConditionFalse,
		},
	}

	conditionTypes := normalizeConditionTypes([]api.ConditionType{api.ConditionTypeDegraded})
	condition.Aggregate(object, api.ConditionTypeReady, conditionTypes...)

	g := NewWithT(t)
	g.Expect(condition.Find(object, string(api.ConditionTypeReady)).Status).Should(Equal(v1.ConditionFalse))

	object.Status.Conditions[1].Status = v1.ConditionTrue
	condition.Aggregate(object, api.ConditionTypeReady, conditionTypes...)
	g.Expect(condition.Find(object, string(api.ConditionTypeReady)).Status).Should(Equal(v1.ConditionTrue))
}

type instanceTestObject struct {
	testObject
}

var testObjectGVK = schema.GroupVersionKind{ //nolint:gochecknoglobals // Shared test type identity.
	Group:   "example.io",
	Version: "v1",
	Kind:    "Component",
}

//nolint:govet // Test object layout mirrors a generated Kubernetes object.
type testObject struct {
	v1.TypeMeta `json:",inline"`

	v1.ObjectMeta `json:"metadata"`

	Status testObjectStatus `json:"status"`
}

//nolint:govet // Test status layout mirrors the optional platform accessors.
type testObjectStatus struct {
	api.Status

	api.ReleaseStatus

	api.PhaseStatus

	Platform *api.PlatformProfile `json:"platform,omitempty"`
}

func (o *testObject) GetStatus() *api.Status {
	return &o.Status.Status
}

func (o *testObject) GetConditions() []api.Condition {
	return o.Status.Conditions
}

func (o *testObject) SetConditions(values []api.Condition) {
	o.Status.Conditions = values
}

func (o *testObject) GetReleaseStatus() *api.ReleaseStatus {
	return &o.Status.ReleaseStatus
}

func (o *testObject) SetReleaseStatus(value api.ReleaseStatus) {
	o.Status.ReleaseStatus = value
}

func (o *testObject) GetPhaseStatus() *api.PhaseStatus {
	return &o.Status.PhaseStatus
}

func (o *testObject) SetPhaseStatus(value api.PhaseStatus) {
	o.Status.PhaseStatus = value
}

func (o *testObject) GetPlatformProfile() *api.PlatformProfile {
	if o.Status.Platform == nil {
		o.Status.Platform = &api.PlatformProfile{}
	}

	return o.Status.Platform
}

func (o *testObject) SetPlatformProfile(value api.PlatformProfile) {
	o.Status.Platform = value.DeepCopy()
}

func (o *testObject) DeepCopyObject() runtime.Object {
	copyValue := *o
	//nolint:staticcheck // The embedded metadata value must be copied into the test object.
	objectMeta := o.ObjectMeta.DeepCopy()
	copyValue.ObjectMeta = *objectMeta
	copyValue.Status.Conditions = slices.Clone(o.Status.Conditions)
	copyValue.Status.Releases = slices.Clone(o.Status.Releases)
	if o.Status.Platform != nil {
		copyValue.Status.Platform = o.Status.Platform.DeepCopy()
	}

	return &copyValue
}

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(testObjectGVK, &testObject{})
	_ = scheme.AddConversionFunc(
		&unstructured.Unstructured{},
		&testObject{},
		func(from, to any, _ conversion.Scope) error {
			fromObject, ok := from.(*unstructured.Unstructured)
			if !ok {
				return errors.New("unexpected conversion source") //nolint:err113 // Test-only conversion guard.
			}

			toObject, ok := to.(*testObject)
			if !ok {
				return errors.New("unexpected conversion target") //nolint:err113 // Test-only conversion guard.
			}

			return runtime.DefaultUnstructuredConverter.FromUnstructured(
				fromObject.Object,
				toObject,
			)
		},
	)
	_ = scheme.AddConversionFunc(
		&testObject{},
		&testObject{},
		func(from, to any, _ conversion.Scope) error {
			fromObject, ok := from.(*testObject)
			if !ok {
				return errors.New("unexpected typed conversion source") //nolint:err113 // Test-only conversion guard.
			}

			toObject, ok := to.(*testObject)
			if !ok {
				return errors.New("unexpected typed conversion target") //nolint:err113 // Test-only conversion guard.
			}

			*toObject = *fromObject
			//nolint:staticcheck // The test object's metadata is embedded.
			toObject.ObjectMeta = *fromObject.ObjectMeta.DeepCopy()
			toObject.Status.Conditions = slices.Clone(fromObject.Status.Conditions)
			toObject.Status.Releases = slices.Clone(fromObject.Status.Releases)
			if fromObject.Status.Platform != nil {
				toObject.Status.Platform = fromObject.Status.Platform.DeepCopy()
			}

			return nil
		},
	)
	return scheme
}

func testClient(object client.Object) client.Client {
	scheme := testScheme()
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&testObject{}).
		WithObjects(object).
		Build()
}

func testObjectInstance(name string) *testObject {
	return &testObject{
		TypeMeta: v1.TypeMeta{
			APIVersion: testObjectGVK.GroupVersion().String(),
			Kind:       testObjectGVK.Kind,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:       name,
			Namespace:  "component-system",
			Generation: 3,
			UID:        "component-uid",
		},
		Status: testObjectStatus{
			Platform: &api.PlatformProfile{},
		},
	}
}
