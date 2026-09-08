package tls

import (
	"crypto/tls"

	configv1 "github.com/openshift/api/config/v1"
	ocpcrypto "github.com/openshift/library-go/pkg/crypto"
)

// ConfigFromProfile returns a tls.Config mutator and unsupported cipher names.
func ConfigFromProfile(profile configv1.TLSProfileSpec) (func(*tls.Config), []string) {
	minVersion, err := ocpcrypto.TLSVersion(string(profile.MinTLSVersion))
	if err != nil {
		minVersion = tls.VersionTLS12
	}

	cipherSuites, unsupportedCiphers := cipherCodes(profile.Ciphers)
	curves := curveIDs(profile.Groups)

	return func(configuration *tls.Config) {
		configuration.MinVersion = minVersion
		if len(curves) > 0 {
			configuration.CurvePreferences = curves
		}

		if minVersion != tls.VersionTLS13 {
			configuration.CipherSuites = cipherSuites
		}
	}, unsupportedCiphers
}

func cipherCodes(values []string) (codes []uint16, unsupported []string) {
	for _, value := range values {
		code, err := ocpcrypto.CipherSuite(value)
		if err != nil {
			ianaValues := ocpcrypto.OpenSSLToIANACipherSuites([]string{value})
			if len(ianaValues) == 1 {
				code, err = ocpcrypto.CipherSuite(ianaValues[0])
			}
		}

		if err != nil {
			unsupported = append(unsupported, value)
			continue
		}

		codes = append(codes, code)
	}

	return codes, unsupported
}

func curveIDs(groups []configv1.TLSGroup) []tls.CurveID {
	curves := make([]tls.CurveID, 0, len(groups))
	for _, group := range groups {
		curve, ok := curveID(group)
		if ok {
			curves = append(curves, curve)
		}
	}

	return curves
}

func curveID(group configv1.TLSGroup) (tls.CurveID, bool) {
	switch group {
	case configv1.TLSGroupX25519:
		return tls.X25519, true
	case configv1.TLSGroupSecP256r1:
		return tls.CurveP256, true
	case configv1.TLSGroupSecP384r1:
		return tls.CurveP384, true
	case configv1.TLSGroupSecP521r1:
		return tls.CurveP521, true
	case configv1.TLSGroupX25519MLKEM768:
		return tls.X25519MLKEM768, true
	case configv1.TLSGroupSecP256r1MLKEM768:
		return tls.SecP256r1MLKEM768, true
	case configv1.TLSGroupSecP384r1MLKEM1024:
		return tls.SecP384r1MLKEM1024, true
	default:
		return 0, false
	}
}
