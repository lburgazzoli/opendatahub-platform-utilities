package tls_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/openshift/tls"
)

func TestLoadReadsAPIServerProfile(t *testing.T) {
	t.Parallel()

	modern := *configv1.TLSProfiles[configv1.TLSProfileModernType]
	reader := fakeTLSReader(t, &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: tls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
		},
	})

	result, err := tls.Load(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Watchable).To(BeTrue())
	g.Expect(result.Spec).To(Equal(modern))
}

func TestLoadFallsBackWhenAPIServerAPIIsMissing(t *testing.T) {
	t.Parallel()

	reader := fakeTLSReader(t)
	result, err := tls.Load(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Watchable).To(BeFalse())
	g.Expect(result.Spec).To(Equal(*configv1.TLSProfiles[configv1.TLSProfileIntermediateType]))
}

func TestLoadKeepsWatcherForTransientAPIServerErrors(t *testing.T) {
	t.Parallel()

	reader := errorReader{err: apierrors.NewServiceUnavailable("api unavailable")}
	result, err := tls.Load(t.Context(), reader)

	g := NewWithT(t)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.Watchable).To(BeTrue())
	g.Expect(result.Spec).To(Equal(*configv1.TLSProfiles[configv1.TLSProfileIntermediateType]))
}

func fakeTLSReader(t *testing.T, objects ...client.Object) client.Reader {
	t.Helper()

	scheme := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(configv1.Install(scheme)).To(Succeed())

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

type errorReader struct {
	err error
}

func (r errorReader) Get(
	context.Context,
	client.ObjectKey,
	client.Object,
	...client.GetOption,
) error {
	return r.err
}

func (r errorReader) List(
	context.Context,
	client.ObjectList,
	...client.ListOption,
) error {
	return r.err
}
