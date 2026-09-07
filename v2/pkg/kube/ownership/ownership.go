// Package ownership contains Kubernetes owner-reference helpers.
package ownership

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	ErrOwnerRequired = errors.New("owner is required")
	ErrOwnerName     = errors.New("owner name is required")
	ErrOwnerUID      = errors.New("owner UID is required")
	ErrOwnerGVK      = errors.New("owner GVK is required")
)

// OwnerRefFrom creates a controller owner reference from a scheme-registered
// owner object.
func OwnerRefFrom(owner client.Object, scheme *runtime.Scheme) (metav1.OwnerReference, error) {
	if owner == nil {
		return metav1.OwnerReference{}, ErrOwnerRequired
	}

	if owner.GetName() == "" {
		return metav1.OwnerReference{}, ErrOwnerName
	}

	if owner.GetUID() == "" {
		return metav1.OwnerReference{}, ErrOwnerUID
	}

	if scheme == nil {
		return metav1.OwnerReference{}, ErrOwnerGVK
	}

	gvks, _, err := scheme.ObjectKinds(owner)
	if err != nil {
		return metav1.OwnerReference{}, fmt.Errorf("resolve owner GVK: %w", err)
	}

	if len(gvks) == 0 {
		return metav1.OwnerReference{}, ErrOwnerGVK
	}

	controller := true
	blockOwnerDeletion := true

	return metav1.OwnerReference{
		APIVersion:         gvks[0].GroupVersion().String(),
		Kind:               gvks[0].Kind,
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		Controller:         &controller,
		BlockOwnerDeletion: &blockOwnerDeletion,
	}, nil
}

// SetControllerReference sets a controller owner reference on object.
func SetControllerReference(owner, object client.Object, scheme *runtime.Scheme) error {
	if owner == nil || object == nil {
		return ErrOwnerRequired
	}

	if scheme == nil {
		return ErrOwnerGVK
	}

	return controllerutil.SetControllerReference(owner, object, scheme)
}

// ControlledBy reports whether object has the supplied controller owner.
func ControlledBy(object, owner client.Object) bool {
	if object == nil || owner == nil {
		return false
	}

	return metav1.IsControlledBy(object, owner)
}

// OwnedBy reports whether object has any owner reference with the owner's UID.
func OwnedBy(object, owner client.Object) bool {
	if object == nil || owner == nil {
		return false
	}

	for _, reference := range object.GetOwnerReferences() {
		if reference.UID == owner.GetUID() {
			return true
		}
	}

	return false
}
