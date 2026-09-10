package reconciler

import (
	"fmt"
	"strings"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	platformhandler "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/handler"
	platformpredicate "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/predicate"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler/dynamicwatcher"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type watchRegistration struct {
	object    client.Object
	options   WatchOptions
	ownerOnly bool
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
			configured.EventHandler = platformhandler.AnnotationToNameClusterScoped(b.options.InstanceAnnotation)
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
		platformpredicate.PartOfWithLabel(
			b.options.PartOfLabel,
			strings.ToLower(instance.GetObjectKind().GroupVersionKind().Kind),
		),
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
