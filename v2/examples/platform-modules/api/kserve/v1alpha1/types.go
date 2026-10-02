package v1alpha1

import (
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	platformv1alpha1 "github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/api/platform/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GroupVersion = schema.GroupVersion{Group: "kserve.example.odh.io", Version: "v1alpha1"} //nolint:gochecknoglobals

var KserveGVK = GroupVersion.WithKind("Kserve") //nolint:gochecknoglobals // GVK is an API constant.

// Kserve is the first illustrative module CR.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="Kserve must be named cluster"
type Kserve struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   platformv1alpha1.ComponentSpec `json:"spec,omitempty"`
	Status platformv1alpha1.ModuleStatus  `json:"status,omitempty"`
}

// KserveList contains Kserve objects.
// +kubebuilder:object:root=true
type KserveList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Kserve `json:"items"`
}

func NewKserve() *Kserve {
	return &Kserve{TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: KserveGVK.Kind}}
}

func (k *Kserve) GetStatus() *platformapi.Status {
	return &k.Status.Status
}

func (k *Kserve) GetReleaseStatus() *platformapi.ReleaseStatus {
	return &k.Status.ReleaseStatus
}

func (k *Kserve) SetReleaseStatus(status platformapi.ReleaseStatus) {
	k.Status.ReleaseStatus = status
}

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, new(Kserve), new(KserveList))
	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}
