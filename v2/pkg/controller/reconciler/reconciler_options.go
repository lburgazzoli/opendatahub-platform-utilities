package reconciler

import (
	"errors"
	"slices"
	"time"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	"k8s.io/apimachinery/pkg/runtime/schema"
	crpredicate "sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	defaultCleanupTimeout = 10 * time.Minute
	DefaultFinalizerName  = "platform.opendatahub.io/finalizer"
)

var (
	ErrManagerRequired          = errors.New("reconciler manager is required")
	ErrPrototypeRequired        = errors.New("reconciler prototype is required")
	ErrCleanupTimeout           = errors.New("reconciler cleanup timeout cannot be negative")
	ErrPrototypeCopy            = errors.New("reconciler prototype deep copy is invalid")
	ErrStatusRequired           = errors.New("reconciler status is required")
	ErrConditionManagerRequired = errors.New("reconciler condition manager is required")
)

func defaultOptions() Options {
	return Options{
		CleanupTimeout: new(defaultCleanupTimeout),
	}
}

// Options configures reconciler policy.
//
//nolint:govet // The exported option layout follows its documented policy groups.
type Options struct {
	ControllerName                    string
	FieldOwner                        string
	DefaultRequeueAfter               time.Duration
	DynamicOwnership                  bool
	CleanupTimeout                    *time.Duration
	PlatformProfile                   *api.PlatformProfile
	ExcludeFromOwnership              []schema.GroupVersionKind
	DynamicOwnershipDefaultPredicates []crpredicate.Predicate
	DynamicOwnershipGVKPredicates     map[schema.GroupVersionKind][]crpredicate.Predicate
	ConditionManager                  ConditionManagerFactory
}

// ApplyTo applies a complete option value to target while preserving absent
// pointer values and cloning caller-owned profile data.
func (o Options) ApplyTo(target *Options) {
	target.ControllerName = o.ControllerName
	target.FieldOwner = o.FieldOwner

	if o.CleanupTimeout != nil {
		target.CleanupTimeout = new(*o.CleanupTimeout)
	}

	target.DefaultRequeueAfter = o.DefaultRequeueAfter
	target.DynamicOwnership = o.DynamicOwnership
	target.ExcludeFromOwnership = slices.Clone(o.ExcludeFromOwnership)
	target.DynamicOwnershipDefaultPredicates = slices.Clone(o.DynamicOwnershipDefaultPredicates)
	target.DynamicOwnershipGVKPredicates = clonePredicateMap(o.DynamicOwnershipGVKPredicates)
	target.ConditionManager = o.ConditionManager

	if o.PlatformProfile == nil {
		target.PlatformProfile = nil
		return
	}

	target.PlatformProfile = o.PlatformProfile.DeepCopy()
}

// Option configures a reconciler.
type Option = option.Option[Options]

// WithControllerName sets the controller-runtime controller name.
func WithControllerName(name string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ControllerName = name
	})
}

// WithFieldOwner sets the status server-side-apply field owner.
func WithFieldOwner(owner string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.FieldOwner = owner
	})
}

// WithCleanupTimeout sets the deletion cleanup timeout. Zero disables it.
func WithCleanupTimeout(timeout time.Duration) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.CleanupTimeout = new(timeout)
	})
}

// WithDefaultRequeueAfter sets the default successful-reconciliation schedule.
func WithDefaultRequeueAfter(after time.Duration) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.DefaultRequeueAfter = after
	})
}

// WithDynamicOwnership enables framework-managed dynamic watches.
func WithDynamicOwnership() Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.DynamicOwnership = true
	})
}

// WithProfile projects a defensive copy of the startup profile into opted-in status.
func WithProfile(profile api.PlatformProfile) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.PlatformProfile = profile.DeepCopy()
	})
}

// WithPlatformProfile is the descriptive alias used by controller setup.
func WithPlatformProfile(profile api.PlatformProfile) Option {
	return WithProfile(profile)
}

// WithExcludedOwnershipTypes excludes exact GVKs from dynamic ownership.
func WithExcludedOwnershipTypes(types ...schema.GroupVersionKind) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ExcludeFromOwnership = append(options.ExcludeFromOwnership, types...)
	})
}

// WithDynamicOwnershipDefaultPredicates configures the fallback predicates for
// framework-managed owned watches.
func WithDynamicOwnershipDefaultPredicates(values ...crpredicate.Predicate) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.DynamicOwnershipDefaultPredicates = append(options.DynamicOwnershipDefaultPredicates, values...)
	})
}

// WithDynamicOwnershipGVKPredicates configures predicates for exact owned
// resource GVKs. These take precedence over all default policies.
func WithDynamicOwnershipGVKPredicates(
	values map[schema.GroupVersionKind][]crpredicate.Predicate,
) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.DynamicOwnershipGVKPredicates = clonePredicateMap(values)
	})
}

func clonePredicateMap(
	values map[schema.GroupVersionKind][]crpredicate.Predicate,
) map[schema.GroupVersionKind][]crpredicate.Predicate {
	if values == nil {
		return nil
	}

	cloned := make(map[schema.GroupVersionKind][]crpredicate.Predicate, len(values))
	for gvk, predicates := range values {
		cloned[gvk] = slices.Clone(predicates)
	}

	return cloned
}

// WithConditionManagerFactory configures reconciler-owned condition status
// processing.
func WithConditionManagerFactory(factory ConditionManagerFactory) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ConditionManager = factory
	})
}
