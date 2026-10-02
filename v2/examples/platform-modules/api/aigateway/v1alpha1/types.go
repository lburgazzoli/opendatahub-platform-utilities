package v1alpha1

import (
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	platformv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GroupVersion = schema.GroupVersion{Group: "aigateway.example.odh.io", Version: "v1alpha1"} //nolint:gochecknoglobals

var AIGatewayGVK = GroupVersion.WithKind("AIGateway") //nolint:gochecknoglobals // GVK is an API constant.

// AIGateway is the second illustrative module CR.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="AIGateway must be named cluster"
type AIGateway struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   platformv1alpha1.ComponentSpec `json:"spec,omitempty"`
	Status platformv1alpha1.ModuleStatus  `json:"status,omitempty"`
}

// AIGatewayList contains AIGateway objects.
// +kubebuilder:object:root=true
type AIGatewayList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []AIGateway `json:"items"`
}

func (a *AIGateway) GetStatus() *platformapi.Status {
	return &a.Status.Status
}

func (a *AIGateway) GetReleaseStatus() *platformapi.ReleaseStatus {
	return &a.Status.ReleaseStatus
}

func (a *AIGateway) SetReleaseStatus(status platformapi.ReleaseStatus) {
	a.Status.ReleaseStatus = status
}

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, new(AIGateway), new(AIGatewayList))
	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}
