package tls_test

import (
	"testing"

	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/openshift/tls"
)

func TestProfileSpecFromSecurityProfile(t *testing.T) {
	t.Parallel()

	modern := configv1.TLSProfiles[configv1.TLSProfileModernType]
	cases := map[string]struct {
		profile *configv1.TLSSecurityProfile
		want    configv1.TLSProfileSpec
	}{
		"missing profile": {
			want: *configv1.TLSProfiles[configv1.TLSProfileIntermediateType],
		},
		"named profile": {
			profile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
			want:    *modern,
		},
		"custom profile": {
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS13,
						Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
					},
				},
			},
			want: configv1.TLSProfileSpec{
				MinTLSVersion: configv1.VersionTLS13,
				Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
			},
		},
	}

	for name, test := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			g := NewWithT(t)
			got := tls.ProfileSpecFromSecurityProfile(test.profile)

			g.Expect(got).NotTo(BeNil())
			g.Expect(*got).To(Equal(test.want))
		})
	}
}

func TestFromProfileFloorsUnsupportedVersion(t *testing.T) {
	t.Parallel()

	profile := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
		Custom: &configv1.CustomTLSProfile{
			TLSProfileSpec: configv1.TLSProfileSpec{
				MinTLSVersion: configv1.VersionTLS11,
				Ciphers:       []string{"unsupported"},
			},
		},
	}

	minVersion, cipherSuites := tls.FromProfile(t.Context(), profile, tls.FormatShort)

	g := NewWithT(t)
	g.Expect(minVersion).To(Equal("TLS1.2"))
	g.Expect(cipherSuites).NotTo(BeEmpty())
}
