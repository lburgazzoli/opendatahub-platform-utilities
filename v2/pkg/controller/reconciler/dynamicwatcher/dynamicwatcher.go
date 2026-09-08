// Package dynamicwatcher owns framework-managed watches for published resources.
package dynamicwatcher

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sync"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	platformpredicate "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
	kubeGVK "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	kubeMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	crhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Watcher registers each required GVK at most once.
//
//nolint:govet // The fields are grouped by watch topology and synchronized state.
type Watcher struct {
	controller        controller.Controller
	cache             cache.Cache
	primaryGVK        schema.GroupVersionKind
	primaryMapping    *kubeMeta.RESTMapping
	annotationHandler crhandler.MapFunc

	mu         sync.RWMutex
	registered sets.Set[schema.GroupVersionKind]
}

var (
	ErrControllerRequired = errors.New("dynamic watcher controller is required")
	ErrCacheRequired      = errors.New("dynamic watcher cache is required")
	ErrMapperRequired     = errors.New("dynamic watcher REST mapper is required")
)

// New creates a dynamic watch synchronizer.
func New(
	controllerInstance controller.Controller,
	cacheInstance cache.Cache,
	mapper kubeMeta.RESTMapper,
	primaryGVK schema.GroupVersionKind,
	staticGVKs ...schema.GroupVersionKind,
) (*Watcher, error) {
	if controllerInstance == nil {
		return nil, ErrControllerRequired
	}

	if cacheInstance == nil {
		return nil, ErrCacheRequired
	}

	if mapper == nil {
		return nil, ErrMapperRequired
	}

	primaryMapping, err := mapper.RESTMapping(primaryGVK.GroupKind(), primaryGVK.Version)
	if err != nil {
		return nil, fmt.Errorf("resolve primary mapping %s: %w", primaryGVK, err)
	}

	registered := sets.New(primaryGVK)
	registered.Insert(staticGVKs...)
	annotationHandler := handler.EnqueueByOwnerAnnotation()

	return &Watcher{
		controller:        controllerInstance,
		cache:             cacheInstance,
		primaryGVK:        primaryGVK,
		primaryMapping:    primaryMapping,
		annotationHandler: annotationHandler,
		registered:        registered,
	}, nil
}

// Sync publishes watches for all resource types in the current snapshot.
func (w *Watcher) Sync(
	ctx context.Context,
	accessor resources.Accessor,
	excluded []schema.GroupVersionKind,
) error {
	if w == nil || accessor == nil {
		return nil
	}

	excluded = append(slices.Clone(excluded), kubeGVK.Namespace)

	for _, object := range accessor.All() {
		err := w.syncObject(ctx, object, excluded)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Watcher) syncObject(
	ctx context.Context,
	object *unstructured.Unstructured,
	excluded []schema.GroupVersionKind,
) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	if object == nil {
		return nil
	}

	gvk := object.GroupVersionKind()
	if gvk.Empty() {
		return nil
	}

	if slices.Contains(excluded, gvk) {
		return nil
	}

	err := w.register(gvk)
	if err != nil {
		return err
	}

	return nil
}

func (w *Watcher) register(gvk schema.GroupVersionKind) error {
	w.mu.RLock()
	registered := w.registered.Has(gvk)
	w.mu.RUnlock()

	if registered {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.registered.Has(gvk) {
		return nil
	}

	var mapFunc crhandler.MapFunc

	switch gvk {
	case kubeGVK.CustomResourceDefinition:
		mapFunc = w.annotationHandler
	default:
		mapFunc = w.requestMapper
	}

	watchSource := source.TypedKind[client.Object, reconcile.Request](
		w.cache,
		resources.GvkToUnstructured(gvk),
		crhandler.EnqueueRequestsFromMapFunc(mapFunc),
		platformpredicate.Funcs{
			CreateFunc: func(event.CreateEvent) bool { return true },
			UpdateFunc: func(value event.UpdateEvent) bool {
				if value.ObjectOld == nil || value.ObjectNew == nil {
					return false
				}

				return value.ObjectOld.GetGeneration() != value.ObjectNew.GetGeneration() ||
					!maps.Equal(value.ObjectOld.GetLabels(), value.ObjectNew.GetLabels()) ||
					!maps.Equal(value.ObjectOld.GetAnnotations(), value.ObjectNew.GetAnnotations())
			},
		},
	)

	err := w.controller.Watch(watchSource)
	if err != nil {
		return fmt.Errorf("register dynamic watch for %s: %w", gvk, err)
	}

	w.registered.Insert(gvk)
	return nil
}

func (w *Watcher) requestMapper(
	ctx context.Context,
	object client.Object,
) []reconcile.Request {
	if object == nil {
		return nil
	}

	owner := metav1.GetControllerOf(object)
	if owner == nil {
		return w.annotationHandler(ctx, object)
	}

	ownerGVK := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
	if ownerGVK != w.primaryGVK || owner.Name == "" {
		return w.annotationHandler(ctx, object)
	}

	namespace := ""
	if w.primaryMapping.Scope.Name() == kubeMeta.RESTScopeNameNamespace {
		namespace = object.GetNamespace()
	}

	return []reconcile.Request{{NamespacedName: types.NamespacedName{
		Namespace: namespace,
		Name:      owner.Name,
	}}}
}
