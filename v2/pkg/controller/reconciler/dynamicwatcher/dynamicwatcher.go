// Package dynamicwatcher owns framework-managed watches for published resources.
package dynamicwatcher

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	platformpredicate "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
	kubeGVK "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	kubeMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	crhandler "sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// EventHandler and MapFunc keep the reconciler watch option surface independent
// of the controller-runtime handler package while retaining its contracts.
type EventHandler = crhandler.EventHandler
type MapFunc = crhandler.MapFunc
type Predicate = predicate.Predicate

// DynamicPredicate controls whether a configured watch is installed.
type DynamicPredicate func(context.Context, *pipeline.Request) (bool, error)

// Registration describes one conditional watch delegated by the reconciler.
type Registration struct {
	Object            client.Object
	EventHandler      crhandler.EventHandler
	Predicates        []predicate.Predicate
	DynamicPredicates []DynamicPredicate
}

// OwnershipOptions configures predicates for framework-managed owned watches.
//
//nolint:govet // Predicate policy slices and maps are intentionally grouped.
type OwnershipOptions struct {
	DefaultPredicates []predicate.Predicate
	GVKPredicates     map[schema.GroupVersionKind][]predicate.Predicate
}

type watchRoute uint8

const (
	routePrimary watchRoute = iota
	routeConfigured
	routeOwned
	routeUnowned
	routeCRD
)

type watchKey struct {
	gvk   schema.GroupVersionKind
	name  string
	route watchRoute
}

// Watcher registers each required watch at most once. The registry is
// read-dominant because every reconciliation observes already-known GVKs.
//
//nolint:govet // The fields are grouped by topology and synchronized state.
type Watcher struct {
	controller        controller.Controller
	cache             cache.Cache
	mapper            kubeMeta.RESTMapper
	primaryGVK        schema.GroupVersionKind
	primaryMapping    *kubeMeta.RESTMapping
	annotationHandler crhandler.MapFunc
	registrations     []Registration
	ownership         OwnershipOptions

	mu         sync.RWMutex
	registered sets.Set[watchKey]
}

var (
	ErrControllerRequired = errors.New("dynamic watcher controller is required")
	ErrCacheRequired      = errors.New("dynamic watcher cache is required")
	ErrMapperRequired     = errors.New("dynamic watcher REST mapper is required")
	ErrObjectGVKRequired  = errors.New("dynamic watcher object GVK is required")
)

// New creates a dynamic watch synchronizer. Static owned GVKs seed the
// registry so framework-managed ownership does not duplicate builder watches.
func New(
	controllerInstance controller.Controller,
	cacheInstance cache.Cache,
	mapper kubeMeta.RESTMapper,
	primaryGVK schema.GroupVersionKind,
	staticGVKs ...schema.GroupVersionKind,
) (*Watcher, error) {
	return newWatcher(
		controllerInstance,
		cacheInstance,
		mapper,
		primaryGVK,
		staticGVKs,
		nil,
		OwnershipOptions{DefaultPredicates: nil, GVKPredicates: nil},
	)
}

// NewWithRegistrations creates a dynamic watch synchronizer with conditional
// builder registrations that are evaluated after a reconciliation.
func NewWithRegistrations(
	controllerInstance controller.Controller,
	cacheInstance cache.Cache,
	mapper kubeMeta.RESTMapper,
	primaryGVK schema.GroupVersionKind,
	staticGVKs []schema.GroupVersionKind,
	registrations []Registration,
	ownership OwnershipOptions,
) (*Watcher, error) {
	return newWatcher(controllerInstance, cacheInstance, mapper, primaryGVK, staticGVKs, registrations, ownership)
}

func newWatcher(
	controllerInstance controller.Controller,
	cacheInstance cache.Cache,
	mapper kubeMeta.RESTMapper,
	primaryGVK schema.GroupVersionKind,
	staticGVKs []schema.GroupVersionKind,
	registrations []Registration,
	ownership OwnershipOptions,
) (*Watcher, error) {
	switch {
	case controllerInstance == nil:
		return nil, ErrControllerRequired
	case cacheInstance == nil:
		return nil, ErrCacheRequired
	case mapper == nil:
		return nil, ErrMapperRequired
	}

	primaryMapping, err := mapper.RESTMapping(primaryGVK.GroupKind(), primaryGVK.Version)
	if err != nil {
		return nil, fmt.Errorf("resolve primary mapping %s: %w", primaryGVK, err)
	}

	registered := sets.New[watchKey](watchKey{gvk: primaryGVK, route: routePrimary, name: ""})
	for _, gvk := range staticGVKs {
		registered.Insert(watchKey{gvk: gvk, route: routeOwned, name: ""})
	}

	return &Watcher{
		controller:        controllerInstance,
		cache:             cacheInstance,
		mapper:            mapper,
		primaryGVK:        primaryGVK,
		primaryMapping:    primaryMapping,
		annotationHandler: handler.EnqueueByOwnerAnnotation(),
		registrations:     slices.Clone(registrations),
		ownership:         cloneOwnershipOptions(ownership),
		registered:        registered,
	}, nil
}

// SyncConfigured evaluates conditional Watches and Owns registrations.
func (w *Watcher) SyncConfigured(ctx context.Context, request *pipeline.Request) error {
	if w == nil || request == nil {
		return nil
	}

	for _, registration := range w.registrations {
		if registration.Object == nil {
			return fmt.Errorf("register dynamic watch: %w", ErrObjectGVKRequired)
		}

		gvk := registration.Object.GetObjectKind().GroupVersionKind()
		if gvk.Empty() {
			return fmt.Errorf("register dynamic watch: %T: %w", registration.Object, ErrObjectGVKRequired)
		}

		key := watchKey{gvk: gvk, route: routeConfigured, name: ""}
		if w.isRegistered(key) {
			continue
		}

		shouldRegister, err := evaluate(ctx, request, registration.DynamicPredicates)
		if err != nil {
			return fmt.Errorf("evaluate dynamic watch condition: %w", err)
		}
		if !shouldRegister {
			continue
		}

		err = w.registerWatch(
			key,
			registration.Object,
			registration.EventHandler,
			registration.Predicates,
		)
		if err != nil {
			return fmt.Errorf("register configured dynamic watch for %s: %w", gvk, err)
		}
	}

	return nil
}

func evaluate(
	ctx context.Context,
	request *pipeline.Request,
	predicates []DynamicPredicate,
) (bool, error) {
	for _, dynamicPredicate := range predicates {
		if dynamicPredicate == nil {
			continue
		}

		ok, err := dynamicPredicate(ctx, request)
		if err != nil || !ok {
			return ok, err
		}
	}

	return true, nil
}

// Sync publishes ownership watches for all resource types in the current
// snapshot. It also preserves the legacy CRD and unmanaged-resource routes.
func (w *Watcher) Sync(
	ctx context.Context,
	request *pipeline.Request,
	excluded []schema.GroupVersionKind,
) error {
	if w == nil || request == nil || request.Resources == nil {
		return nil
	}

	excluded = append(slices.Clone(excluded), kubeGVK.Namespace)
	seen := sets.New[watchKey]()

	for _, object := range request.Resources.All() {
		err := w.syncObject(ctx, request, object, excluded, seen)
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *Watcher) syncObject(
	ctx context.Context,
	request *pipeline.Request,
	object *unstructured.Unstructured,
	excluded []schema.GroupVersionKind,
	seen sets.Set[watchKey],
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
	if gvk.Empty() || slices.Contains(excluded, gvk) {
		return nil
	}

	if gvk == kubeGVK.CustomResourceDefinition {
		key := watchKey{gvk: gvk, route: routeCRD, name: object.GetName()}
		if seen.Has(key) || w.isRegistered(key) {
			return nil
		}

		seen.Insert(key)
		return w.registerCRD(request, object.GetName())
	}

	owned := resources.GetAnnotation(object, platformannotations.ManagedByODHOperator) != "false"
	key := watchKey{gvk: gvk, route: routeUnowned, name: ""}
	if owned {
		key.route = routeOwned
	}

	if seen.Has(key) || w.isRegistered(key) {
		return nil
	}

	seen.Insert(key)

	available, err := w.apiAvailable(gvk)
	if err != nil {
		return fmt.Errorf("check API availability for %s: %w", gvk, err)
	}
	if !available {
		return nil
	}

	eventHandler := handler.ToNamed(instanceName(request))
	watchPredicates := []predicate.Predicate{platformpredicate.DeleteEvent()}

	if owned {
		key.route = routeOwned
		eventHandler = w.ownerHandler(request)
		if eventHandler == nil {
			eventHandler = crhandler.EnqueueRequestsFromMapFunc(w.requestMapper)
		}
		watchPredicates = []predicate.Predicate{w.ownerPredicate(gvk)}
	}

	return w.registerWatch(key, resources.GvkToUnstructured(gvk), eventHandler, watchPredicates)
}

func (w *Watcher) registerCRD(request *pipeline.Request, name string) error {
	return w.registerWatch(
		watchKey{gvk: kubeGVK.CustomResourceDefinition, route: routeCRD, name: name},
		resources.GvkToUnstructured(kubeGVK.CustomResourceDefinition),
		handler.ToNamed(instanceName(request)),
		[]predicate.Predicate{platformpredicate.CreatedOrUpdatedOrDeletedNamed(name)},
	)
}

func instanceName(request *pipeline.Request) string {
	if request == nil || request.Instance == nil {
		return ""
	}

	return request.Instance.GetName()
}

func (w *Watcher) ownerHandler(request *pipeline.Request) crhandler.EventHandler {
	if request == nil || request.Client == nil {
		return nil
	}

	return crhandler.EnqueueRequestForOwner(
		request.Client.Scheme(),
		request.Client.RESTMapper(),
		resources.GvkToUnstructured(w.primaryGVK),
		crhandler.OnlyControllerOwner(),
	)
}

func (w *Watcher) ownerPredicate(gvk schema.GroupVersionKind) predicate.Predicate {
	values := w.ownerPredicates(gvk)
	return predicate.And(values...)
}

func (w *Watcher) ownerPredicates(gvk schema.GroupVersionKind) []predicate.Predicate {
	if values, found := w.ownership.GVKPredicates[gvk]; found {
		return slices.Clone(values)
	}

	switch {
	case gvk == kubeGVK.Deployment:
		return []predicate.Predicate{platformpredicate.DefaultDeploymentPredicate}
	case len(w.ownership.DefaultPredicates) > 0:
		return slices.Clone(w.ownership.DefaultPredicates)
	default:
		return []predicate.Predicate{platformpredicate.DefaultPredicate}
	}
}

func (w *Watcher) isRegistered(key watchKey) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.registered.Has(key)
}

func cloneOwnershipOptions(options OwnershipOptions) OwnershipOptions {
	cloned := OwnershipOptions{
		DefaultPredicates: slices.Clone(options.DefaultPredicates),
		GVKPredicates:     nil,
	}
	if options.GVKPredicates == nil {
		return cloned
	}

	cloned.GVKPredicates = make(map[schema.GroupVersionKind][]predicate.Predicate, len(options.GVKPredicates))
	for gvk, values := range options.GVKPredicates {
		cloned.GVKPredicates[gvk] = slices.Clone(values)
	}

	return cloned
}

func (w *Watcher) apiAvailable(gvk schema.GroupVersionKind) (bool, error) {
	_, err := w.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	switch {
	case err == nil:
		return true, nil
	case kubeMeta.IsNoMatchError(err):
		return false, nil
	default:
		return false, err
	}
}

func (w *Watcher) registerWatch(
	key watchKey,
	object client.Object,
	eventHandler crhandler.EventHandler,
	watchPredicates []predicate.Predicate,
) error {
	w.mu.RLock()
	registered := w.registered.Has(key)
	w.mu.RUnlock()

	if registered {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if w.registered.Has(key) {
		return nil
	}

	watchSource := source.TypedKind[client.Object, reconcile.Request](
		w.cache,
		object,
		eventHandler,
		watchPredicates...,
	)
	err := w.controller.Watch(watchSource)
	if err != nil {
		return fmt.Errorf("watch registration failed: %w", err)
	}

	w.registered.Insert(key)
	return nil
}

// register retains the focused helper used by unit tests and callers that
// need the framework-managed owner-first route directly.
func (w *Watcher) register(gvk schema.GroupVersionKind) error {
	return w.registerWatch(
		watchKey{gvk: gvk, route: routeOwned, name: ""},
		resources.GvkToUnstructured(gvk),
		crhandler.EnqueueRequestsFromMapFunc(w.requestMapper),
		[]predicate.Predicate{platformpredicate.DefaultPredicate},
	)
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
	if ownerGVK != w.primaryGVK || owner.Name == "" || w.primaryMapping == nil {
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

// EventHandlerFromMap adapts a map function for watch options.
func EventHandlerFromMap(value MapFunc) EventHandler {
	return crhandler.EnqueueRequestsFromMapFunc(value)
}
