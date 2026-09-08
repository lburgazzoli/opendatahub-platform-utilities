package tls_test

import (
	cryptotls "crypto/tls"
	"testing"

	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"

	openshifttls "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/openshift/tls"
)

func TestConfigFromProfile(t *testing.T) {
	t.Parallel()

	configure, unsupported := openshifttls.ConfigFromProfile(configv1.TLSProfileSpec{
		MinTLSVersion: configv1.VersionTLS12,
		Ciphers: []string{
			"ECDHE-RSA-AES128-GCM-SHA256",
			"unsupported-cipher",
		},
		Groups: []configv1.TLSGroup{configv1.TLSGroupSecP256r1},
	})
	configuration := &cryptotls.Config{}
	configure(configuration)

	g := NewWithT(t)
	g.Expect(configuration.MinVersion).To(Equal(uint16(cryptotls.VersionTLS12)))
	g.Expect(configuration.CurvePreferences).To(ConsistOf(cryptotls.CurveP256))
	g.Expect(configuration.CipherSuites).NotTo(BeEmpty())
	g.Expect(unsupported).To(ConsistOf("unsupported-cipher"))
}
