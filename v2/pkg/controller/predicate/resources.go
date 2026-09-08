package predicate

import (
	appsv1 "k8s.io/api/apps/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

// DeploymentStatusChanged accepts deployment generation or replica status changes.
func DeploymentStatusChanged() crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			oldReplicas, oldReady, oldOK := deploymentStatus(value.ObjectOld)
			newReplicas, newReady, newOK := deploymentStatus(value.ObjectNew)

			return oldOK && newOK &&
				(value.ObjectOld.GetGeneration() != value.ObjectNew.GetGeneration() ||
					oldReplicas != newReplicas || oldReady != newReady)
		},
	}
}

// ConfigMapDataChanged accepts updates to ConfigMap data.
func ConfigMapDataChanged() crpredicate.Predicate {
	return dataChanged(gvk.ConfigMap, "data")
}

// SecretDataChanged accepts updates to Secret data.
func SecretDataChanged() crpredicate.Predicate {
	return dataChanged(gvk.Secret, "data")
}

func dataChanged(gvk schema.GroupVersionKind, field string) crpredicate.Predicate {
	return Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(value event.UpdateEvent) bool {
			oldData, oldOK := nestedField(value.ObjectOld, gvk, field)
			newData, newOK := nestedField(value.ObjectNew, gvk, field)

			return oldOK && newOK && !apiequality.Semantic.DeepEqual(oldData, newData)
		},
	}
}

func deploymentStatus(object client.Object) (int64, int64, bool) {
	if object == nil {
		return 0, 0, false
	}

	switch typed := object.(type) {
	case *appsv1.Deployment:
		return int64(typed.Status.Replicas), int64(typed.Status.ReadyReplicas), true
	case *unstructured.Unstructured:
		if typed.GroupVersionKind() != gvk.Deployment {
			return 0, 0, false
		}

		replicas, _, replicasErr := unstructured.NestedInt64(typed.Object, "status", "replicas")
		ready, _, readyErr := unstructured.NestedInt64(typed.Object, "status", "readyReplicas")
		return replicas, ready, replicasErr == nil && readyErr == nil
	default:
		return 0, 0, false
	}
}

func nestedField(object client.Object, gvk schema.GroupVersionKind, field string) (any, bool) {
	u, ok := object.(*unstructured.Unstructured)
	if !ok || u.GroupVersionKind() != gvk {
		return nil, false
	}

	value, found, err := unstructured.NestedFieldNoCopy(u.Object, field)
	return value, found && err == nil
}
