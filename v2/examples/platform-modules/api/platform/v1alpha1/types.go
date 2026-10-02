package v1alpha1

import (
	platformapi "github.com/opendatahub-io/odh-platform-utilities/v2/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var GroupVersion = schema.GroupVersion{Group: "example.platform.odh.io", Version: "v1alpha1"} //nolint:gochecknoglobals

var (
	PlatformGVK       = GroupVersion.WithKind("Platform")       //nolint:gochecknoglobals // GVK is an API constant.
	PlatformModuleGVK = GroupVersion.WithKind("PlatformModule") //nolint:gochecknoglobals // GVK is an API constant.
	ServingGVK        = GroupVersion.WithKind("Serving")        //nolint:gochecknoglobals // GVK is an API constant.
)

const (
	PlatformName = "default-platform"
	InstanceName = "cluster"

	// ConditionModulesReady reports whether every selected PlatformModule is ready.
	ConditionModulesReady platformapi.ConditionType = "ModulesReady"
)

type ControllerStatus struct {
	platformapi.ReleaseStatus `json:",inline"`
	platformapi.Status        `json:",inline"`
}

// ResourceRef identifies a resource rendered for one PlatformModule.
type ResourceRef struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace,omitempty"`
	Name       string `json:"name"`
}

// PlatformModuleStatus records the resources managed for one module.
//
//nolint:govet // Keep the shared status embedded before the inventory field.
type PlatformModuleStatus struct {
	ControllerStatus `json:",inline"`

	Resources []ResourceRef `json:"resources,omitempty"`
}

// Platform declares the enabled module names.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default-platform'",message="invalid Platform name"
type Platform struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlatformSpec     `json:"spec,omitempty"`
	Status ControllerStatus `json:"status,omitempty"`
}

type PlatformSpec struct {
	// +listType=set
	Modules []string `json:"modules,omitempty"`
}

// PlatformList contains Platform objects.
// +kubebuilder:object:root=true
type PlatformList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Platform `json:"items"`
}

func NewPlatform() *Platform {
	return &Platform{TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: PlatformGVK.Kind}}
}

func (p *Platform) GetStatus() *platformapi.Status {
	return &p.Status.Status
}

func (p *Platform) GetReleaseStatus() *platformapi.ReleaseStatus {
	return &p.Status.ReleaseStatus
}

func (p *Platform) SetReleaseStatus(status platformapi.ReleaseStatus) {
	p.Status.ReleaseStatus = status
}

// PlatformModule binds a module name to its controller deployment.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == self.spec.module",message="name must equal module"
type PlatformModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PlatformModuleSpec   `json:"spec,omitempty"`
	Status PlatformModuleStatus `json:"status,omitempty"`
}

type PlatformModuleSpec struct {
	Module string `json:"module"`
}

// PlatformModuleList contains PlatformModule objects.
// +kubebuilder:object:root=true
type PlatformModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PlatformModule `json:"items"`
}

func NewPlatformModule() *PlatformModule {
	return &PlatformModule{TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: PlatformModuleGVK.Kind}}
}

func (p *PlatformModule) GetStatus() *platformapi.Status {
	return &p.Status.Status
}

func (p *PlatformModule) GetReleaseStatus() *platformapi.ReleaseStatus {
	return &p.Status.ReleaseStatus
}

func (p *PlatformModule) SetReleaseStatus(status platformapi.ReleaseStatus) {
	p.Status.ReleaseStatus = status
}

type ComponentSpec struct {
	// +kubebuilder:default=Removed
	// +kubebuilder:validation:Enum=Managed;Removed
	ManagementState platformapi.ManagementState `json:"managementState,omitempty"`
}

type ModuleStatus struct {
	platformapi.ReleaseStatus `json:",inline"`
	platformapi.Status        `json:",inline"`
}

// Serving selects the Kserve and MaaS module states.
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'cluster'",message="Serving must be named cluster"
type Serving struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ServingSpec   `json:"spec,omitempty"`
	Status ServingStatus `json:"status,omitempty"`
}

type ServingSpec struct {
	Kserve ComponentSpec `json:"kserve,omitempty"`
	MaaS   ComponentSpec `json:"maas,omitempty"`
}

type ServingStatus struct {
	Kserve *ServingModuleStatus `json:"kserve,omitempty"`
	MaaS   *ServingModuleStatus `json:"maas,omitempty"`

	ControllerStatus `json:",inline"` //nolint:embeddedstructfieldcheck // Pointer fields lead for lower GC scan cost.
}

type ServingModuleStatus struct {
	Ready bool `json:"ready"`
}

// ServingList contains Serving objects.
// +kubebuilder:object:root=true
type ServingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Serving `json:"items"`
}

func NewServing() *Serving {
	return &Serving{TypeMeta: metav1.TypeMeta{APIVersion: GroupVersion.String(), Kind: ServingGVK.Kind}}
}

func (s *Serving) GetStatus() *platformapi.Status {
	return &s.Status.Status
}

func (s *Serving) GetReleaseStatus() *platformapi.ReleaseStatus {
	return &s.Status.ReleaseStatus
}

func (s *Serving) SetReleaseStatus(status platformapi.ReleaseStatus) {
	s.Status.ReleaseStatus = status
}

func AddToScheme(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		new(Platform), new(PlatformList),
		new(PlatformModule), new(PlatformModuleList),
		new(Serving), new(ServingList),
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}
