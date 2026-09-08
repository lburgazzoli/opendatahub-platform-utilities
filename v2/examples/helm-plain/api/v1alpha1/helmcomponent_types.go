package v1alpha1

import (
	"errors"

	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

//nolint:gochecknoglobals // scheme registration requires a package-level group version.
var GroupVersion = schema.GroupVersion{Group: "examples.odh.io", Version: "v1alpha1"}

//nolint:gochecknoglobals // the example API exposes its primary GVK.
var ComponentGVK = GroupVersion.WithKind("HelmComponent")

var ErrRequestInstance = errors.New("request instance has unexpected type")

// HelmComponent is the primary object reconciled by the example controller.
//
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type HelmComponent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HelmComponentSpec   `json:"spec,omitempty"`
	Status HelmComponentStatus `json:"status,omitempty"`
}

type HelmComponentSpec struct {
	platformapi.ManagementSpec `json:",inline"`
}

type HelmComponentStatus struct {
	platformapi.ReleaseStatus `json:",inline"`
	platformapi.Status        `json:",inline"`
}

func NewHelmComponent() *HelmComponent {
	component := new(HelmComponent)
	component.TypeMeta = metav1.TypeMeta{
		APIVersion: GroupVersion.String(),
		Kind:       ComponentGVK.Kind,
	}
	component.Spec.ManagementSpec = platformapi.ManagementSpec{
		ManagementState: platformapi.ManagementStateManaged,
	}

	return component
}

func (c *HelmComponent) GetStatus() *platformapi.Status {
	return &c.Status.Status
}

func (c *HelmComponent) GetConditions() []platformapi.Condition {
	return c.Status.Conditions
}

func (c *HelmComponent) SetConditions(conditions []platformapi.Condition) {
	c.Status.Conditions = conditions
}

func (c *HelmComponent) GetReleaseStatus() *platformapi.ReleaseStatus {
	return &c.Status.ReleaseStatus
}

func (c *HelmComponent) SetReleaseStatus(status platformapi.ReleaseStatus) {
	c.Status.ReleaseStatus = status
}

// HelmComponentList contains HelmComponent objects.
//
// +kubebuilder:object:generate=true
// +kubebuilder:object:root=true
type HelmComponentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []HelmComponent `json:"items"`
}

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion, new(HelmComponent), new(HelmComponentList))
	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}
