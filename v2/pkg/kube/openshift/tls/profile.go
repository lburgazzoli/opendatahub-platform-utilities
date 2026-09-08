package tls

import (
	"context"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	ocpcrypto "github.com/openshift/library-go/pkg/crypto"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// APIServerName is the well-known name of the OpenShift APIServer object.
const APIServerName = "cluster"

// VersionFormat controls the output format of TLS version strings.
type VersionFormat int

const (
	// FormatShort returns values such as TLS1.2.
	FormatShort VersionFormat = iota
	// FormatGo returns values such as VersionTLS12.
	FormatGo
)

func intermediateSpec() configv1.TLSProfileSpec {
	return *configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
}

// ProfileSpecFromSecurityProfile resolves a security profile to a concrete
// profile, falling back to the OpenShift Intermediate profile when the input
// is absent or unsupported.
func ProfileSpecFromSecurityProfile(profile *configv1.TLSSecurityProfile) *configv1.TLSProfileSpec {
	if profile == nil {
		profile = &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType}
	}

	switch profile.Type {
	case configv1.TLSProfileCustomType:
		if profile.Custom != nil {
			return &profile.Custom.TLSProfileSpec
		}
	case configv1.TLSProfileOldType, configv1.TLSProfileIntermediateType, configv1.TLSProfileModernType:
		if spec := configv1.TLSProfiles[profile.Type]; spec != nil {
			return spec
		}
	}

	spec := intermediateSpec()
	return &spec
}

func minVersionToShort(value configv1.TLSProtocolVersion) string {
	switch value {
	case configv1.VersionTLS12:
		return "TLS1.2"
	case configv1.VersionTLS13:
		return "TLS1.3"
	default:
		return ""
	}
}

func minVersionToGo(value configv1.TLSProtocolVersion) string {
	switch value {
	case configv1.VersionTLS12:
		return "VersionTLS12"
	case configv1.VersionTLS13:
		return "VersionTLS13"
	default:
		return ""
	}
}

// MinVersionFromSpec returns the minimum TLS version in the requested format.
func MinVersionFromSpec(ctx context.Context, spec *configv1.TLSProfileSpec, format VersionFormat) string {
	logger := logf.FromContext(ctx).WithName("MinVersionFromSpec")
	minVersion := configv1.TLSProfiles[configv1.TLSProfileIntermediateType].MinTLSVersion

	if spec != nil && spec.MinTLSVersion != "" {
		minVersion = spec.MinTLSVersion
	}

	var name string
	switch format {
	case FormatGo:
		name = minVersionToGo(minVersion)
	default:
		name = minVersionToShort(minVersion)
	}

	if name != "" {
		return name
	}

	logger.V(1).Info("unsupported minimum TLS version; using TLS 1.2", "minVersion", minVersion)
	if format == FormatGo {
		return "VersionTLS12"
	}

	return "TLS1.2"
}

// CipherSuitesFromSpec returns IANA cipher names supported by Go.
func CipherSuitesFromSpec(ctx context.Context, spec *configv1.TLSProfileSpec) string {
	logger := logf.FromContext(ctx).WithName("CipherSuitesFromSpec")
	if spec == nil {
		defaultSpec := intermediateSpec()
		spec = &defaultSpec
	}

	ianaCiphers := ocpcrypto.OpenSSLToIANACipherSuites(spec.Ciphers)
	if len(ianaCiphers) == 0 {
		logger.V(1).Info("no supported cipher suites; using Intermediate profile")
		defaultSpec := intermediateSpec()
		ianaCiphers = ocpcrypto.OpenSSLToIANACipherSuites(defaultSpec.Ciphers)
	}

	return strings.Join(ianaCiphers, ",")
}

// IsVersionSupported reports whether a profile version maps to a supported
// proxy flag value.
func IsVersionSupported(value configv1.TLSProtocolVersion) bool {
	return minVersionToShort(value) != ""
}

// FromProfile resolves a security profile to proxy-compatible version and
// cipher-suite values.
func FromProfile(
	ctx context.Context,
	profile *configv1.TLSSecurityProfile,
	format VersionFormat,
) (string, string) {
	spec := ProfileSpecFromSecurityProfile(profile)
	if !IsVersionSupported(spec.MinTLSVersion) {
		fallback := intermediateSpec()
		spec = &fallback
	}

	return MinVersionFromSpec(ctx, spec, format), CipherSuitesFromSpec(ctx, spec)
}
