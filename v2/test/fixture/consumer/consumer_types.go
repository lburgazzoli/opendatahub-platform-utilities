// Package consumer contains a standalone generated-consumer fixture.
// +kubebuilder:object:generate=true
package consumer

import (
	api "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Consumer is the generated-consumer fixture root object.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
type Consumer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsumerSpec   `json:"spec,omitempty"`
	Status ConsumerStatus `json:"status,omitempty"`
}

// ConsumerSpec is deliberately small; the fixture exists to exercise
// consumer-owned CR generation independently from the implementation module.
type ConsumerSpec struct {
	ManagementState string `json:"managementState,omitempty"`
}

// ConsumerStatus represents the status fields used by the contract fixture.
type ConsumerStatus struct {
	api.ReleaseStatus `json:",inline"`
	api.Status        `json:",inline"`
}

func (c *Consumer) GetStatus() *api.Status { return &c.Status.Status }

func (c *Consumer) GetReleaseStatus() *api.ReleaseStatus { return &c.Status.ReleaseStatus }

func (c *Consumer) SetReleaseStatus(status api.ReleaseStatus) { c.Status.ReleaseStatus = status }

// ConsumerList is the generated-consumer fixture list object.
// +kubebuilder:object:root=true
type ConsumerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Consumer `json:"items"`
}

// OptionalConsumer is a fixture with every optional status capability.
// +kubebuilder:object:root=true
//
//nolint:govet // keep Kubernetes embedded fields ahead of regular CR fields
type OptionalConsumer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConsumerSpec           `json:"spec,omitempty"`
	Status OptionalConsumerStatus `json:"status,omitempty"`
}

// OptionalConsumerStatus composes the required and optional v2 status types.
//
//nolint:govet // keep embedded API status fragments together for CR generation
type OptionalConsumerStatus struct {
	api.PhaseStatus   `json:",inline"`
	api.ReleaseStatus `json:",inline"`
	api.Status        `json:",inline"`

	Platform *api.PlatformProfile `json:"platform,omitempty"`
}

func (c *OptionalConsumer) GetStatus() *api.Status { return &c.Status.Status }

func (c *OptionalConsumer) GetConditions() []api.Condition { return c.Status.Conditions }

func (c *OptionalConsumer) SetConditions(conditions []api.Condition) {
	c.Status.Conditions = conditions
}

func (c *OptionalConsumer) GetReleaseStatus() *api.ReleaseStatus { return &c.Status.ReleaseStatus }

func (c *OptionalConsumer) SetReleaseStatus(status api.ReleaseStatus) {
	c.Status.ReleaseStatus = status
}

func (c *OptionalConsumer) GetPhaseStatus() *api.PhaseStatus { return &c.Status.PhaseStatus }

func (c *OptionalConsumer) SetPhaseStatus(status api.PhaseStatus) { c.Status.PhaseStatus = status }

func (c *OptionalConsumer) GetPlatformProfile() *api.PlatformProfile { return c.Status.Platform }

func (c *OptionalConsumer) SetPlatformProfile(profile api.PlatformProfile) {
	c.Status.Platform = &profile
}
