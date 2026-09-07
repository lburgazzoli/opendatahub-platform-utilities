package api

// PlatformKind identifies the platform product.
type PlatformKind string

// DistributionKind identifies the Kubernetes distribution.
type DistributionKind string

// Distribution describes the hosting distribution.
//
// +kubebuilder:object:generate=true
type Distribution struct {
	Kind    DistributionKind `json:"kind"`
	Version string           `json:"version,omitempty"`
}

// PlatformProfile is an immutable startup snapshot of platform information.
//
// +kubebuilder:object:generate=true
type PlatformProfile struct {
	Annotations  map[string]string `json:"annotations,omitempty"`
	Distribution Distribution      `json:"distribution"`
	Kind         PlatformKind      `json:"kind"`
	Version      string            `json:"version,omitempty"`
}
