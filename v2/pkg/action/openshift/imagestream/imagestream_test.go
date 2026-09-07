package imagestream_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/openshift/imagestream"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

func TestRunReportsFailedImageStreamImports(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	stream := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "image.openshift.io/v1",
		"kind":       "ImageStream",
		"metadata": map[string]any{
			"name":      "images",
			"namespace": "ns",
			"labels":    map[string]any{"managed": "true"},
		},
		"status": map[string]any{
			"tags": []any{map[string]any{
				"tag":   "latest",
				"items": []any{},
				"conditions": []any{map[string]any{
					"type":    "ImportSuccess",
					"status":  "False",
					"message": "registry unavailable",
				}},
			}},
		},
	}}
	cli := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithObjects(stream).Build()

	observation, err := imagestream.New(
		imagestream.WithSelectorLabels(map[string]string{"managed": "true"}),
	).Run(t.Context(), imagestream.RunOptions{Client: cli, Namespace: "ns"})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(observation.Failed).Should(Equal(1))
	g.Expect(observation.Condition.Status).Should(Equal(metav1.ConditionFalse))
	g.Expect(observation.Condition.Message).Should(ContainSubstring("images:latest"))
}

func TestNewRequiresSelector(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := imagestream.New()

	g.Expect(action.Validate()).Should(MatchError(imagestream.ErrSelectorRequired))
}

func TestExecuteStoresHealthyConditionWhenImageStreamsAreUnavailable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	instance := &platformObject{ConfigMap: corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: "ns"}}}
	request := &pipeline.Request{
		Client:   fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build(),
		Instance: instance,
	}

	action := imagestream.New(imagestream.WithSelectorLabels(map[string]string{"managed": "true"}))
	err := action.Execute(t.Context(), request)

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(condition.IsTrue(instance, imagestream.DefaultConditionType)).Should(BeTrue())
}

func TestRunTreatsOnlyNoMatchAsUnavailable(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	reader := errorReader{err: &meta.NoResourceMatchError{
		PartialResource: schema.GroupVersionResource{Group: "image.openshift.io", Version: "v1", Resource: "imagestreams"},
	}}

	observation, err := imagestream.New(
		imagestream.WithSelectorLabels(map[string]string{"managed": "true"}),
	).Run(t.Context(), imagestream.RunOptions{Client: reader, Namespace: "ns"})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(observation.Condition.Status).Should(Equal(metav1.ConditionTrue))
}

func TestRunPropagatesNotFound(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	reader := errorReader{err: apierrors.NewNotFound(
		schema.GroupResource{Group: "image.openshift.io", Resource: "imagestreams"},
		"ns",
	)}

	_, err := imagestream.New(
		imagestream.WithSelectorLabels(map[string]string{"managed": "true"}),
	).Run(t.Context(), imagestream.RunOptions{Client: reader, Namespace: "ns"})

	g.Expect(err).Should(MatchError(ContainSubstring("list ImageStreams")))
	g.Expect(apierrors.IsNotFound(err)).Should(BeTrue())
}

func TestRunRejectsMalformedStatus(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	stream := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "image.openshift.io/v1",
		"kind":       "ImageStream",
		"metadata": map[string]any{
			"name":      "images",
			"namespace": "ns",
			"labels":    map[string]any{"managed": "true"},
		},
		"status": map[string]any{"tags": "malformed"},
	}}
	cli := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithObjects(stream).Build()

	_, err := imagestream.New(
		imagestream.WithSelectorLabels(map[string]string{"managed": "true"}),
	).Run(t.Context(), imagestream.RunOptions{Client: cli, Namespace: "ns"})

	g.Expect(err).Should(MatchError(ContainSubstring("status.tags")))
}

func TestRunBoundsReportedFailureMessages(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	objects := make([]client.Object, 0, 12)
	for index := range 12 {
		objects = append(objects, &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "image.openshift.io/v1",
			"kind":       "ImageStream",
			"metadata": map[string]any{
				"name":      fmt.Sprintf("images-%d", index),
				"namespace": "ns",
				"labels":    map[string]any{"managed": "true"},
			},
			"status": map[string]any{
				"tags": []any{map[string]any{
					"tag":   fmt.Sprintf("tag-%d", index),
					"items": []any{},
					"conditions": []any{map[string]any{
						"type":    "ImportSuccess",
						"status":  "False",
						"message": "failed",
					}},
				}},
			},
		}})
	}
	cli := fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithObjects(objects...).Build()

	observation, err := imagestream.New(
		imagestream.WithSelectorLabels(map[string]string{"managed": "true"}),
	).Run(t.Context(), imagestream.RunOptions{Client: cli, Namespace: "ns"})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(observation.Failed).Should(Equal(12))
	g.Expect(observation.Condition.Message).Should(ContainSubstring("and 2 more"))
	g.Expect(strings.Count(observation.Condition.Message, "(failed)")).Should(Equal(10))
}

func TestRunRequiresInputs(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := imagestream.New(imagestream.WithSelectorLabels(map[string]string{"managed": "true"}))

	_, err := action.Run(t.Context(), imagestream.RunOptions{Namespace: "ns"})

	g.Expect(err).Should(MatchError(imagestream.ErrClientRequired))
}

func TestExecuteRejectsMissingRequest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := imagestream.New(imagestream.WithSelectorLabels(map[string]string{"managed": "true"}))

	err := action.Execute(t.Context(), nil)

	g.Expect(err).Should(MatchError(imagestream.ErrRequestRequired))
}

type errorReader struct{ err error }

func (r errorReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return r.err
}

func (r errorReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return r.err
}

//nolint:govet // test fixture layout mirrors the embedded platform object.
type platformObject struct {
	corev1.ConfigMap

	conditions []api.Condition
	status     api.Status
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
