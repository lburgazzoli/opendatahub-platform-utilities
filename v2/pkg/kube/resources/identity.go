package resources

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ErrNilObject            = errors.New("resource object is required")
	ErrMissingIdentity      = errors.New("resource identity is incomplete")
	ErrUnregisteredGVK      = errors.New("resource type is not registered in scheme")
	ErrResourceKindRequired = errors.New("resource kind is required")
)

// Identity identifies a Kubernetes object independent of its concrete Go
// representation.
type Identity struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

// String returns the group, version, kind, optional namespace, and name as a path.
func (identity Identity) String() string {
	path := identity.GVK.GroupVersion().String() + "/" + identity.GVK.Kind
	if identity.Namespace != "" {
		path += "/" + identity.Namespace
	}

	return path + "/" + identity.Name
}

// IdentityOf returns the validated identity of an object.
func IdentityOf(object client.Object, scheme *runtime.Scheme) (Identity, error) {
	if object == nil {
		return Identity{}, ErrNilObject
	}

	gvk, err := EnsureGroupVersionKind(scheme, object)
	if err != nil {
		return Identity{}, err
	}

	if object.GetName() == "" {
		return Identity{}, fmt.Errorf("%s: %w", gvk, ErrMissingIdentity)
	}

	return Identity{GVK: gvk, Namespace: object.GetNamespace(), Name: object.GetName()}, nil
}

// EnsureGroupVersionKind resolves and sets a missing GVK using the scheme.
func EnsureGroupVersionKind(scheme *runtime.Scheme, object client.Object) (schema.GroupVersionKind, error) {
	if object == nil {
		return schema.GroupVersionKind{}, ErrNilObject
	}

	current := object.GetObjectKind().GroupVersionKind()
	if !current.Empty() {
		return current, nil
	}

	if scheme == nil {
		return schema.GroupVersionKind{}, ErrUnregisteredGVK
	}

	gvks, _, err := scheme.ObjectKinds(object)
	if err != nil {
		return schema.GroupVersionKind{}, fmt.Errorf("resolve GVK for %T: %w: %w", object, ErrUnregisteredGVK, err)
	}

	if len(gvks) == 0 {
		return schema.GroupVersionKind{}, fmt.Errorf("resolve GVK for %T: %w", object, ErrUnregisteredGVK)
	}

	object.GetObjectKind().SetGroupVersionKind(gvks[0])

	return gvks[0], nil
}
