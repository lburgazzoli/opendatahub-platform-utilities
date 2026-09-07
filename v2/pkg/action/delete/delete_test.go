package deletion_test

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
	deleteaction "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/delete"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

func TestRunDeletesMatchingNamespacedResources(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	matching := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name:      "matching",
		Namespace: "ns",
		Labels:    map[string]string{"managed": "true"},
	}}
	other := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "ns"}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(restMapper).WithObjects(matching, other).Build()
	action := deleteaction.New(
		[]client.Object{&corev1.ConfigMap{}},
		deleteaction.WithLabels(map[string]string{"managed": "true"}),
		deleteaction.WithNamespace("ns"),
	)

	_, err := action.Run(t.Context(), deleteaction.RunOptions{Client: cli})
	g.Expect(err).ShouldNot(HaveOccurred())

	g.Expect(cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "matching"}, &corev1.ConfigMap{})).Should(
		MatchError(ContainSubstring("not found")),
	)
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "other"}, &corev1.ConfigMap{})).Should(Succeed())
}

func TestRunRejectsUnboundedDeletionBeforeClientUse(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := deleteaction.New([]client.Object{&corev1.ConfigMap{}})

	_, err := action.Run(t.Context())

	g.Expect(err).Should(MatchError(deleteaction.ErrSelector))
}

func TestNewRejectsTypedNilPrototype(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	var prototype *corev1.ConfigMap
	action := deleteaction.New([]client.Object{prototype}, deleteaction.WithDeleteAll())

	g.Expect(action.Validate()).Should(MatchError(deleteaction.ErrTypeInvalid))
}

func TestExecuteRejectsMissingRequest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := deleteaction.New([]client.Object{&corev1.ConfigMap{}}, deleteaction.WithDeleteAll())

	err := action.Execute(t.Context(), nil)

	g.Expect(err).Should(MatchError(deleteaction.ErrRequestRequired))
}

func TestRunRequiresNamespaceForNamespacedResources(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(restMapper).Build()

	_, err := deleteaction.New([]client.Object{&corev1.ConfigMap{}}, deleteaction.WithDeleteAll()).Run(
		t.Context(),
		deleteaction.RunOptions{Client: cli},
	)
	g.Expect(err).Should(MatchError(deleteaction.ErrNamespace))
}

func TestExecuteUsesInstanceNamespace(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).Should(Succeed())
	restMapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	restMapper.Add(corev1.SchemeGroupVersion.WithKind("ConfigMap"), meta.RESTScopeNamespace)
	matching := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "matching", Namespace: "ns"}}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRESTMapper(restMapper).WithObjects(matching).Build()
	instance := &platformObject{ConfigMap: corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns"}}}

	err := deleteaction.New([]client.Object{&corev1.ConfigMap{}}, deleteaction.WithDeleteAll()).Execute(
		t.Context(),
		&pipeline.Request{Client: cli, Instance: instance},
	)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(cli.Get(t.Context(), client.ObjectKey{Namespace: "ns", Name: "matching"}, &corev1.ConfigMap{})).Should(
		MatchError(ContainSubstring("not found")),
	)
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
