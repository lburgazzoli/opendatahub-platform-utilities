package reconciler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	platformhandler "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	platformpredicate "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler/dynamicwatcher"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type watchRegistration struct {
	object    client.Object
	options   WatchOptions
	ownerOnly bool
}

// Builder configures a controller topology and its action pipeline.
type Builder struct {
	manager           manager.Manager
	prototype         api.PlatformObject
	options           Options
	pipeline          *pipeline.Pipeline
	watch             []watchRegistration
	rawSources        []source.Source
	predicates        []predicate.Predicate
	cleanupRegistered bool
}

// For starts a reconciler builder for a primary object type.
func For(
	mgr manager.Manager,
	prototype api.PlatformObject,
	options ...Option,
) *Builder {
	configured := defaultOptions()
	for _, optionValue := range options {
		if optionValue != nil {
			optionValue.ApplyTo(&configured)
		}
	}

	return &Builder{
		manager:   mgr,
		prototype: prototype,
		options:   configured,
		pipeline:  pipeline.New(),
		predicates: []predicate.Predicate{predicate.Or(
			predicate.GenerationChangedPredicate{}, //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
			predicate.LabelChangedPredicate{},      //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
			predicate.AnnotationChangedPredicate{}, //nolint:exhaustruct_v5 // zero value retains controller-runtime defaults.
		)},
	}
}

// WithBeforeAction registers an action in the before phase.
func (b *Builder) WithBeforeAction(value pipeline.Action, options ...pipeline.ActionOption) *Builder {
	b.pipeline = b.pipeline.WithBeforeAction(value, options...)
	return b
}

// WithBeforeActionFunc registers a function in the before phase.
func (b *Builder) WithBeforeActionFunc(
	execute func(context.Context, *pipeline.Request) error,
	options ...pipeline.ActionOption,
) *Builder {
	return b.WithBeforeAction(pipeline.Wrap(execute), options...)
}

// WithAction registers an action in the main phase.
func (b *Builder) WithAction(value pipeline.Action, options ...pipeline.ActionOption) *Builder {
	b.pipeline = b.pipeline.WithAction(value, options...)
	return b
}

// WithActionFunc registers a function in the main phase.
func (b *Builder) WithActionFunc(
	execute func(context.Context, *pipeline.Request) error,
	options ...pipeline.ActionOption,
) *Builder {
	return b.WithAction(pipeline.Wrap(execute), options...)
}

// WithAfterAction registers an action in the after phase.
func (b *Builder) WithAfterAction(value pipeline.Action, options ...pipeline.ActionOption) *Builder {
	b.pipeline = b.pipeline.WithAfterAction(value, options...)
	return b
}

// WithAfterActionFunc registers a function in the after phase.
func (b *Builder) WithAfterActionFunc(
	execute func(context.Context, *pipeline.Request) error,
	options ...pipeline.ActionOption,
) *Builder {
	return b.WithAfterAction(pipeline.Wrap(execute), options...)
}

// WithCleanupAction registers an action in the deletion cleanup phase.
func (b *Builder) WithCleanupAction(value pipeline.Action, options ...pipeline.ActionOption) *Builder {
	b.pipeline = b.pipeline.WithCleanupAction(value, options...)
	b.cleanupRegistered = true
	return b
}

// WithCleanupActionFunc registers a function in the cleanup phase.
func (b *Builder) WithCleanupActionFunc(
	execute func(context.Context, *pipeline.Request) error,
	options ...pipeline.ActionOption,
) *Builder {
	return b.WithCleanupAction(pipeline.Wrap(execute), options...)
}

// WithPlatformProfile projects a startup profile into opted-in status.
func (b *Builder) WithPlatformProfile(profile api.PlatformProfile) *Builder {
	b.options.PlatformProfile = profile.DeepCopy()
	return b
}

// WithDynamicOwnership enables framework-managed watches for published resources.
func (b *Builder) WithDynamicOwnership() *Builder {
	b.options.DynamicOwnership = true
	return b
}

// WithControllerName sets the controller-runtime name.
func (b *Builder) WithControllerName(name string) *Builder {
	b.options.ControllerName = name
	return b
}

// WithCleanupTimeout sets the cleanup deadline policy.
func (b *Builder) WithCleanupTimeout(timeout time.Duration) *Builder {
	b.options.CleanupTimeout = new(timeout)
	return b
}

// Watches registers a secondary resource watch.
func (b *Builder) Watches(
	object client.Object,
	options ...WatchOption,
) *Builder {
	b.watch = append(b.watch, b.newWatchRegistration(object, false, options...))
	return b
}

// Owns registers a controller-owner watch for a secondary resource.
func (b *Builder) Owns(object client.Object, options ...WatchOption) *Builder {
	b.watch = append(b.watch, b.newWatchRegistration(object, true, options...))
	return b
}

// WatchesGVK registers a secondary resource watch from its complete GVK.
func (b *Builder) WatchesGVK(gvk schema.GroupVersionKind, options ...WatchOption) *Builder {
	return b.Watches(resources.GvkToUnstructured(gvk), options...)
}

// OwnsGVK registers a controller-owner watch from its complete GVK.
func (b *Builder) OwnsGVK(gvk schema.GroupVersionKind, options ...WatchOption) *Builder {
	return b.Owns(resources.GvkToUnstructured(gvk), options...)
}

// WatchesRawSource registers a controller-runtime raw source.
func (b *Builder) WatchesRawSource(value source.Source) *Builder {
	b.rawSources = append(b.rawSources, value)
	return b
}

// WithEventFilter adds a predicate to the primary watch.
func (b *Builder) WithEventFilter(value predicate.Predicate) *Builder {
	b.predicates = append(b.predicates, value)
	return b
}

// Build validates the topology and registers the controller with the manager.
func (b *Builder) Build() error {
	instance, err := b.preparePrototype()
	if err != nil {
		return err
	}

	hasDynamicWatches := b.hasDynamicWatches()
	r, err := b.newReconciler(instance, b.options.DynamicOwnership || hasDynamicWatches)
	if err != nil {
		return err
	}

	if !b.options.DynamicOwnership && !hasDynamicWatches {
		_, err = b.registerController(r, instance)
		return err
	}

	staticGVKs, err := b.staticWatchGVKs()
	if err != nil {
		return err
	}

	primaryGVK := instance.GetObjectKind().GroupVersionKind()
	mapper := b.manager.GetRESTMapper()
	if mapper == nil {
		return dynamicwatcher.ErrMapperRequired
	}

	cacheInstance := b.manager.GetCache()
	if cacheInstance == nil {
		return dynamicwatcher.ErrCacheRequired
	}

	_, err = mapper.RESTMapping(primaryGVK.GroupKind(), primaryGVK.Version)
	if err != nil {
		return fmt.Errorf("resolve primary mapping %s: %w", primaryGVK, err)
	}

	dynamicRegistrations, err := b.dynamicRegistrations(instance)
	if err != nil {
		return err
	}

	registeredController, err := b.registerController(r, instance)
	if err != nil {
		return err
	}

	r.dynamic, err = dynamicwatcher.NewWithRegistrations(
		registeredController,
		cacheInstance,
		mapper,
		primaryGVK,
		staticGVKs,
		dynamicRegistrations,
		dynamicwatcher.OwnershipOptions{
			DefaultPredicates: b.options.DynamicOwnershipDefaultPredicates,
			GVKPredicates:     b.options.DynamicOwnershipGVKPredicates,
		},
	)

	return err
}

func (b *Builder) preparePrototype() (api.PlatformObject, error) {
	if b.manager == nil {
		return nil, ErrManagerRequired
	}

	if b.prototype == nil {
		return nil, ErrPrototypeRequired
	}

	instance, err := newInstance(b.prototype)
	if err != nil {
		return nil, err
	}

	_, err = resources.EnsureGroupVersionKind(b.manager.GetScheme(), instance)
	if err != nil {
		return nil, fmt.Errorf("resolve primary GVK: %w", err)
	}

	if b.options.CleanupTimeout == nil || *b.options.CleanupTimeout < 0 {
		return nil, ErrCleanupTimeout
	}

	if b.options.ControllerName == "" {
		b.options.ControllerName = instance.GetObjectKind().GroupVersionKind().Kind
	}
	if b.options.FieldOwner == "" {
		b.options.FieldOwner = b.options.ControllerName
	}

	err = validation.Validate(instance)
	if err != nil {
		return nil, fmt.Errorf("validate reconciler prototype: %w", err)
	}

	return instance, nil
}

func (b *Builder) newReconciler(instance api.PlatformObject, hasDynamicWatches bool) (*Reconciler, error) {
	//nolint:staticcheck // The event recorder interface matches this package's event contract.
	recorder := b.manager.GetEventRecorderFor(b.options.ControllerName)
	r := &Reconciler{
		client:            b.manager.GetClient(),
		scheme:            b.manager.GetScheme(),
		prototype:         instance,
		pipeline:          b.pipeline,
		options:           b.options,
		recorder:          recorder,
		cleanupRegistered: b.cleanupRegistered,
	}

	if hasDynamicWatches {
		r.pipeline = r.pipeline.WithAfterAction(pipeline.ActionFunc{
			ActionName: "synchronize-dynamic-watches",
			ExecuteFunc: func(ctx context.Context, request *pipeline.Request) error {
				if r.dynamic == nil {
					return nil
				}

				err := r.dynamic.SyncConfigured(ctx, request)
				if err != nil {
					return err
				}

				if !r.options.DynamicOwnership {
					return nil
				}

				return r.dynamic.Sync(ctx, request, r.options.ExcludeFromOwnership)
			},
		})
	}

	err := r.pipeline.Validate()
	if err != nil {
		return nil, err
	}

	return r, nil
}

func (b *Builder) registerController(
	r *Reconciler,
	instance api.PlatformObject,
) (controller.Controller, error) {
	controllerBuilder := ctrl.NewControllerManagedBy(b.manager).
		Named(b.options.ControllerName).
		For(instance, builder.WithPredicates(b.predicates...))

	for _, registration := range b.watch {
		if registration.options.Dynamic {
			continue
		}

		watchOptions, err := b.watchOptions(registration, instance)
		if err != nil {
			return nil, err
		}

		watchObject, err := b.normalizeWatchObject(registration.object)
		if err != nil {
			return nil, err
		}

		controllerBuilder = controllerBuilder.Watches(
			watchObject,
			watchOptions.EventHandler,
			builder.WithPredicates(watchOptions.Predicates...),
		)
	}

	for _, rawSource := range b.rawSources {
		controllerBuilder = controllerBuilder.WatchesRawSource(rawSource)
	}

	return controllerBuilder.Build(r)
}

func (b *Builder) staticWatchGVKs() ([]schema.GroupVersionKind, error) {
	staticGVKs := make([]schema.GroupVersionKind, 0, len(b.watch))
	for _, registration := range b.watch {
		if !registration.ownerOnly {
			continue
		}

		gvk, err := resources.EnsureGroupVersionKind(
			b.manager.GetScheme(),
			registration.object,
		)
		if err != nil {
			return nil, fmt.Errorf("resolve watch GVK: %w", err)
		}

		staticGVKs = append(staticGVKs, gvk)
	}

	return staticGVKs, nil
}

func (b *Builder) newWatchRegistration(
	object client.Object,
	ownerOnly bool,
	options ...WatchOption,
) watchRegistration {
	configured := WatchOptions{
		EventHandler:      nil,
		Predicates:        nil,
		DynamicPredicates: nil,
		Dynamic:           false,
	}
	for _, optionValue := range options {
		if optionValue != nil {
			optionValue.ApplyTo(&configured)
		}
	}

	return watchRegistration{object: object, ownerOnly: ownerOnly, options: configured}
}

func (b *Builder) hasDynamicWatches() bool {
	for _, registration := range b.watch {
		if registration.options.Dynamic {
			return true
		}
	}

	return false
}

func (b *Builder) watchOptions(
	registration watchRegistration,
	instance api.PlatformObject,
) (WatchOptions, error) {
	configured := registration.options
	_, err := resources.EnsureGroupVersionKind(b.manager.GetScheme(), registration.object)
	if err != nil {
		return WatchOptions{}, fmt.Errorf("resolve watch GVK: %w", err)
	}

	if configured.EventHandler == nil {
		if registration.ownerOnly {
			configured.EventHandler = handler.EnqueueRequestForOwner(
				b.manager.GetScheme(),
				b.manager.GetRESTMapper(),
				instance,
				handler.OnlyControllerOwner(),
			)
		} else {
			configured.EventHandler = platformhandler.AnnotationToNameClusterScoped(platformannotations.InstanceName)
		}
	}

	if len(configured.Predicates) != 0 {
		return configured, nil
	}

	if registration.ownerOnly {
		configured.Predicates = []predicate.Predicate{platformpredicate.DefaultPredicate}
		return configured, nil
	}

	configured.Predicates = []predicate.Predicate{
		platformpredicate.PartOf(strings.ToLower(instance.GetObjectKind().GroupVersionKind().Kind)),
	}
	return configured, nil
}

func (b *Builder) dynamicRegistrations(
	instance api.PlatformObject,
) ([]dynamicwatcher.Registration, error) {
	registrations := make([]dynamicwatcher.Registration, 0, len(b.watch))
	for _, registration := range b.watch {
		if !registration.options.Dynamic {
			continue
		}

		configured, err := b.watchOptions(registration, instance)
		if err != nil {
			return nil, err
		}
		for index, dynamicPredicate := range registration.options.DynamicPredicates {
			if dynamicPredicate == nil {
				return nil, fmt.Errorf("watch condition %d: %w", index, ErrNilWatchPredicate)
			}
		}

		watchObject, err := b.normalizeWatchObject(registration.object)
		if err != nil {
			return nil, err
		}

		registrations = append(registrations, dynamicwatcher.Registration{
			Object:            watchObject,
			EventHandler:      configured.EventHandler,
			Predicates:        configured.Predicates,
			DynamicPredicates: registration.options.DynamicPredicates,
		})
	}

	return registrations, nil
}

func (b *Builder) normalizeWatchObject(object client.Object) (client.Object, error) {
	gvk, err := resources.EnsureGroupVersionKind(b.manager.GetScheme(), object)
	if err != nil {
		return nil, fmt.Errorf("resolve watch GVK: %w", err)
	}

	return resources.GvkToUnstructured(gvk), nil
}
