package tls_test

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/openshift/tls"
)

func TestSecurityProfileWatcherCallsBackOnProfileChange(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(configv1.Install(scheme)).To(Succeed())

	apiServer := &configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: tls.APIServerName},
		Spec: configv1.APIServerSpec{
			TLSSecurityProfile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
		},
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(apiServer).Build()

	var oldProfile configv1.TLSProfileSpec
	var newProfile configv1.TLSProfileSpec
	watcher := &tls.SecurityProfileWatcher{
		Client:                fakeClient,
		InitialTLSProfileSpec: *configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
		OnProfileChange: func(
			_ context.Context,
			oldValue configv1.TLSProfileSpec,
			newValue configv1.TLSProfileSpec,
		) {
			oldProfile = oldValue
			newProfile = newValue
		},
	}

	_, err := watcher.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: tls.APIServerName},
	})

	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(oldProfile).To(Equal(*configv1.TLSProfiles[configv1.TLSProfileIntermediateType]))
	g.Expect(newProfile).To(Equal(*configv1.TLSProfiles[configv1.TLSProfileModernType]))
	g.Expect(watcher.InitialTLSProfileSpec).To(Equal(newProfile))
}

func TestSecurityProfileWatcherIgnoresMissingAPIServer(t *testing.T) {
	t.Parallel()

	scheme := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(configv1.Install(scheme)).To(Succeed())

	watcher := &tls.SecurityProfileWatcher{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	_, err := watcher.Reconcile(t.Context(), ctrl.Request{
		NamespacedName: client.ObjectKey{Name: tls.APIServerName},
	})

	g.Expect(err).NotTo(HaveOccurred())
}
