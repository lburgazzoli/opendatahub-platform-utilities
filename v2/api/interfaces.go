package api

import "sigs.k8s.io/controller-runtime/pkg/client"

// StatusAccessor provides access to the common status block.
type StatusAccessor interface {
	GetStatus() *Status
}

// ConditionsAccessor provides condition access. Status implements this interface.
type ConditionsAccessor interface {
	GetConditions() []Condition
	SetConditions(conditions []Condition)
}

// ReleaseStatusAccessor provides access to release status.
type ReleaseStatusAccessor interface {
	GetReleaseStatus() *ReleaseStatus
	SetReleaseStatus(status ReleaseStatus)
}

// PhaseStatusAccessor is an opt-in phase capability.
type PhaseStatusAccessor interface {
	GetPhaseStatus() *PhaseStatus
	SetPhaseStatus(status PhaseStatus)
}

// PlatformProfileAccessor is an opt-in status projection capability.
type PlatformProfileAccessor interface {
	GetPlatformProfile() *PlatformProfile
	SetPlatformProfile(profile PlatformProfile)
}

// PlatformObject is the minimum contract read by the platform orchestrator.
type PlatformObject interface {
	client.Object
	StatusAccessor
	ReleaseStatusAccessor
}
