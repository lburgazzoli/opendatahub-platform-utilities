// Package handler contains focused event-to-request mappings for controllers.
package handler

import (
	"context"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	crhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Fn adapts a request mapping function to a controller-runtime event handler.
func Fn(fn func(context.Context, client.Object) []reconcile.Request) crhandler.EventHandler {
	return crhandler.EnqueueRequestsFromMapFunc(fn)
}

// RequestFromObject enqueues the object's own namespace and name.
func RequestFromObject() crhandler.EventHandler {
	return Fn(func(_ context.Context, object client.Object) []reconcile.Request {
		if object == nil || object.GetName() == "" {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{
			Namespace: object.GetNamespace(),
			Name:      object.GetName(),
		}}}
	})
}

// ToNamed enqueues a known object name. It is useful for singleton owners.
func ToNamed(name string) crhandler.EventHandler {
	return Fn(func(_ context.Context, _ client.Object) []reconcile.Request {
		if name == "" {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: name}}}
	})
}

// LabelToName enqueues the object named by a label on the watched object.
func LabelToName(key string) crhandler.EventHandler {
	return Fn(func(_ context.Context, object client.Object) []reconcile.Request {
		if object == nil {
			return nil
		}

		return requestForValue(object.GetLabels(), key, object.GetNamespace())
	})
}

// AnnotationToName enqueues the object named by an annotation on the watched object.
func AnnotationToName(key string) crhandler.EventHandler {
	return Fn(func(_ context.Context, object client.Object) []reconcile.Request {
		if object == nil {
			return nil
		}

		return requestForValue(object.GetAnnotations(), key, object.GetNamespace())
	})
}

// AnnotationToNameClusterScoped enqueues an annotated primary with an empty
// namespace. It matches singleton controllers whose primary resource is
// cluster-scoped while the watched resource may be namespaced.
func AnnotationToNameClusterScoped(key string) crhandler.EventHandler {
	return Fn(func(_ context.Context, object client.Object) []reconcile.Request {
		if object == nil {
			return nil
		}

		return requestForValue(object.GetAnnotations(), key, "")
	})
}

// EnqueueByOwnerAnnotation resolves the canonical platform owner annotations.
func EnqueueByOwnerAnnotation() crhandler.MapFunc {
	return func(_ context.Context, object client.Object) []reconcile.Request {
		if object == nil {
			return nil
		}

		name := resources.GetAnnotation(object, platformannotations.InstanceName)
		if name == "" {
			return nil
		}

		return []reconcile.Request{{NamespacedName: types.NamespacedName{
			Namespace: resources.GetAnnotation(object, platformannotations.InstanceNamespace),
			Name:      name,
		}}}
	}
}

func requestForValue(values map[string]string, key string, namespace string) []reconcile.Request {
	if values == nil || key == "" || values[key] == "" {
		return nil
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Namespace: namespace,
		Name:      values[key],
	}}}
}
