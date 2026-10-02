package api

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ManagementState defines whether a component is actively managed.
type ManagementState string

const (
	// ManagementStateManaged requests active reconciliation.
	ManagementStateManaged ManagementState = "Managed"
	// ManagementStateRemoved requests resource removal.
	ManagementStateRemoved ManagementState = "Removed"
)

// ManagementSpec carries the user's management intent.
//
// +kubebuilder:object:generate=true
type ManagementSpec struct {
	// ManagementState controls whether the component is reconciled.
	// +kubebuilder:validation:Enum=Managed;Removed
	// +kubebuilder:default=Managed
	ManagementState ManagementState `json:"managementState,omitempty"`
}

// ConditionSeverity expresses the severity of a condition.
type ConditionSeverity string

const (
	// ConditionSeverityError is the default blocking severity.
	ConditionSeverityError ConditionSeverity = ""
	// ConditionSeverityInfo is informational and non-blocking.
	ConditionSeverityInfo ConditionSeverity = "Info"
)

// ConditionType identifies a condition.
type ConditionType string

const (
	// ConditionTypeReady is the top-level aggregate condition.
	ConditionTypeReady ConditionType = "Ready"
	// ConditionTypeProvisioningSucceeded reflects manifest application.
	ConditionTypeProvisioningSucceeded ConditionType = "ProvisioningSucceeded"
	// ConditionTypeDegraded indicates impaired operation.
	ConditionTypeDegraded ConditionType = "Degraded"
)

// Phase represents the top-level lifecycle phase.
type Phase string

const (
	// PhaseReady indicates full operation.
	PhaseReady Phase = "Ready"
	// PhaseNotReady indicates the component is unavailable.
	PhaseNotReady Phase = "Not Ready"
)

// Condition represents an observation of a module's state.
//
// +kubebuilder:object:generate=true
type Condition struct {
	LastTransitionTime metav1.Time            `json:"lastTransitionTime"`
	LastHeartbeatTime  *metav1.Time           `json:"lastHeartbeatTime,omitempty"`
	Type               string                 `json:"type"`
	Status             metav1.ConditionStatus `json:"status"`
	Reason             string                 `json:"reason,omitempty"`
	Message            string                 `json:"message,omitempty"`
	Severity           ConditionSeverity      `json:"severity,omitempty"`
	ObservedGeneration int64                  `json:"observedGeneration,omitempty"`
}

// Status is the common status block shared by platform objects.
//
// +kubebuilder:object:generate=true
type Status struct {
	Conditions         []Condition `json:"conditions,omitempty"`
	ObservedGeneration int64       `json:"observedGeneration,omitempty"`
}

// GetConditions returns the conditions stored in the common status block.
func (s *Status) GetConditions() []Condition {
	return s.Conditions
}

// SetConditions replaces the conditions stored in the common status block.
func (s *Status) SetConditions(conditions []Condition) {
	s.Conditions = conditions
}

// ComponentRelease describes software release metadata.
//
// +kubebuilder:object:generate=true
type ComponentRelease struct {
	Name    string `json:"name"              yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	RepoURL string `json:"repoUrl,omitempty" yaml:"repoUrl,omitempty"`
}

// ReleaseStatus contains releases reported by a platform object.
//
// +kubebuilder:object:generate=true
type ReleaseStatus struct {
	// +listType=map
	// +listMapKey=name
	Releases []ComponentRelease `json:"releases,omitempty" yaml:"releases,omitempty"`
}

// PhaseStatus stores the optional lifecycle phase.
//
// +kubebuilder:object:generate=true
type PhaseStatus struct {
	Phase Phase `json:"phase,omitempty"`
}
