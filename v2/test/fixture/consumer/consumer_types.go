// Package consumer contains a standalone generated-consumer fixture.
// +kubebuilder:object:generate=true
package consumer

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

// ConsumerList is the generated-consumer fixture list object.
// +kubebuilder:object:root=true
type ConsumerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Consumer `json:"items"`
}
