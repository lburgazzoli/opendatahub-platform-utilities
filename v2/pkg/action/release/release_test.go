package release_test

import (
	"testing"
	"testing/fstest"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	releaseaction "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action/release"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
)

func TestRunReadsAndNormalizesReleaseMetadata(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := releaseaction.New(
		releaseaction.WithFS(fstest.MapFS{
			"metadata.yaml": &fstest.MapFile{
				Data: []byte("releases:\n" +
					"- name: z\n  version: ' 2.0 '\n" +
					"- name: empty\n  version: ''\n" +
					"- name: a\n  version: '1.0'\n"),
			},
		}),
		releaseaction.WithPath("metadata.yaml"),
	)

	status, err := action.Run(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(status.Releases).Should(Equal([]api.ComponentRelease{
		{Name: "a", Version: "1.0"},
		{Name: "z", Version: "2.0"},
	}))
}

func TestRunTreatsMissingMetadataAsEmpty(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	status, err := releaseaction.New(
		releaseaction.WithFS(fstest.MapFS{}),
		releaseaction.WithPath("missing.yaml"),
	).Run(t.Context())
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(status.Releases).Should(BeEmpty())
}

func TestWithPathCanExplicitlyClearTheDefault(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := releaseaction.New(releaseaction.WithPath(""))

	g.Expect(action.Validate()).Should(MatchError(releaseaction.ErrPathRequired))
}

func TestExecuteStoresReleaseStatus(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	instance := &platformObject{}
	action := releaseaction.New(releaseaction.WithFS(fstest.MapFS{
		"metadata.yaml": &fstest.MapFile{Data: []byte("releases:\n- name: component\n  version: '1.0'\n")},
	}), releaseaction.WithPath("metadata.yaml"))

	err := action.Execute(t.Context(), &pipeline.Request{
		Client:   fake.NewClientBuilder().WithScheme(runtime.NewScheme()).Build(),
		Instance: instance,
	})

	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(instance.releases.Releases).Should(Equal([]api.ComponentRelease{{Name: "component", Version: "1.0"}}))
}

func TestExecuteRejectsMissingRequest(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	action := releaseaction.New()

	err := action.Execute(t.Context(), nil)

	g.Expect(err).Should(MatchError(releaseaction.ErrRequestRequired))
}

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
