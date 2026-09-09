package reconciler

import (
	"context"
	"fmt"
	"time"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler/dynamicwatcher"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/validation"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Builder configures a controller topology and its action pipeline.
type Builder struct {
	manager    manager.Manager
	prototype  api.PlatformObject
	options    Options
	pipeline   *pipeline.Pipeline
	watch      []watchRegistration
	rawSources []source.Source
	predicates []predicate.Predicate
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
	configured.ConditionTypes = normalizeConditionTypes(configured.ConditionTypes)

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
		client:    b.manager.GetClient(),
		scheme:    b.manager.GetScheme(),
		prototype: instance,
		pipeline:  b.pipeline,
		options:   b.options,
		recorder:  recorder,
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
