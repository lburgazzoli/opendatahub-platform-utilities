package deploy

import (
	"maps"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	platformmetadata "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
)

// CacheOptions configures the process-local deploy cache. Disabled opts out of
// the cache, which is enabled by default.
type CacheOptions struct {
	TTL      time.Duration
	Disabled bool
}

// Options configures an Action. Maps and slices are copied when options are
// applied, and a non-nil collection replaces the current collection.
//
//nolint:govet // field order follows the public option grouping and readability.
type Options struct {
	ContinueOnError bool
	MetadataPolicy  platformmetadata.Policy
	FieldOwner      FieldOwnerFunc
	Labels          map[string]string
	Annotations     map[string]string
	Sort            SortFunc
	// Cache is enabled by default. A nil value in a complete Options value
	// leaves the default unchanged; Disabled explicitly turns it off.
	Cache                *CacheOptions
	ExcludeFromOwnership []schema.GroupVersionKind
	ManagedAnnotation    string
	ApplyCustomizers     map[schema.GroupVersionKind]CustomizerFunc
}

// ApplyTo applies a complete options value over the current configuration.
//
//nolint:cyclop // each option field has independent documented zero semantics.
func (o Options) ApplyTo(target *Options) {
	if o.MetadataPolicy != nil {
		target.MetadataPolicy = o.MetadataPolicy
	}
	if o.FieldOwner != nil {
		target.FieldOwner = o.FieldOwner
	}
	if o.Labels != nil {
		target.Labels = maps.Clone(o.Labels)
	}
	if o.Annotations != nil {
		target.Annotations = maps.Clone(o.Annotations)
	}
	if o.Sort != nil {
		target.Sort = o.Sort
	}
	if o.Cache != nil {
		cache := *o.Cache
		target.Cache = &cache
	}
	if o.ExcludeFromOwnership != nil {
		target.ExcludeFromOwnership = append([]schema.GroupVersionKind(nil), o.ExcludeFromOwnership...)
	}
	target.ContinueOnError = o.ContinueOnError
	if o.ManagedAnnotation != "" {
		target.ManagedAnnotation = o.ManagedAnnotation
	}
	if o.ApplyCustomizers != nil {
		target.ApplyCustomizers = maps.Clone(o.ApplyCustomizers)
	}
}

// Option configures an Action.
type Option = option.Option[Options]

// WithMetadataPolicy replaces the metadata policy. A nil policy is ignored.
func WithMetadataPolicy(policy platformmetadata.Policy) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if policy != nil {
			options.MetadataPolicy = policy
		}
	})
}

// WithFieldOwner sets a static SSA field owner for every resource.
func WithFieldOwner(owner string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.FieldOwner = func(client.Object) string { return owner }
	})
}

// WithFieldOwnerFunc sets a dynamic SSA field-owner resolver.
func WithFieldOwnerFunc(owner FieldOwnerFunc) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if owner != nil {
			options.FieldOwner = owner
		}
	})
}

// WithLabel adds or replaces one desired-resource label.
func WithLabel(key string, value string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.Labels == nil {
			options.Labels = make(map[string]string)
		}
		options.Labels[key] = value
	})
}

// WithLabels merges desired-resource labels.
func WithLabels(values map[string]string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.Labels == nil {
			options.Labels = make(map[string]string)
		}
		maps.Copy(options.Labels, values)
	})
}

// WithAnnotation adds or replaces one desired-resource annotation.
func WithAnnotation(key string, value string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.Annotations == nil {
			options.Annotations = make(map[string]string)
		}
		options.Annotations[key] = value
	})
}

// WithAnnotations merges desired-resource annotations.
func WithAnnotations(values map[string]string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.Annotations == nil {
			options.Annotations = make(map[string]string)
		}
		maps.Copy(options.Annotations, values)
	})
}

// WithSort replaces the apply-order strategy. A nil sorter is ignored.
func WithSort(sort SortFunc) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if sort != nil {
			options.Sort = sort
		}
	})
}

// WithApplyOrder installs the default dependency ordering.
func WithApplyOrder() Option { return WithSort(ApplyOrder) }

// WithCache enables or disables deploy caching. Optional settings configure
// the TTL; enabled takes precedence over their Disabled field.
func WithCache(enabled bool, values ...*CacheOptions) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		cache := CacheOptions{Disabled: !enabled}
		if len(values) > 0 && values[0] != nil {
			cache = *values[0]
			cache.Disabled = !enabled
		}
		options.Cache = &cache
	})
}

// WithContinueOnError controls whether deployment continues after an
// individual resource fails.
func WithContinueOnError(enabled bool) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ContinueOnError = enabled
	})
}

// WithManagedAnnotation changes the managed-resource opt-out annotation. An
// empty key is ignored.
func WithManagedAnnotation(key string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if key != "" {
			options.ManagedAnnotation = key
		}
	})
}

// WithExcludeFromOwnership adds GVKs that must not receive owner references.
func WithExcludeFromOwnership(gvks ...schema.GroupVersionKind) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ExcludeFromOwnership = append(options.ExcludeFromOwnership, gvks...)
	})
}

// WithApplyCustomizer registers an SSA customizer for one GVK.
func WithApplyCustomizer(gvk schema.GroupVersionKind, customizer CustomizerFunc) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.ApplyCustomizers == nil {
			options.ApplyCustomizers = make(map[schema.GroupVersionKind]CustomizerFunc)
		}
		options.ApplyCustomizers[gvk] = customizer
	})
}

func defaultOptions() Options {
	return Options{
		MetadataPolicy: platformmetadata.DefaultPolicy(),
		FieldOwner: func(owner client.Object) string {
			return owner.GetObjectKind().GroupVersionKind().Kind
		},
		Sort:                 ApplyOrder,
		Cache:                &CacheOptions{},
		ManagedAnnotation:    annotations.ManagedByODHOperator,
		ExcludeFromOwnership: []schema.GroupVersionKind{kubegvk.Namespace},
		ApplyCustomizers: map[schema.GroupVersionKind]CustomizerFunc{
			kubegvk.ClusterRole:            applyAggregatedClusterRoleCustomizer,
			kubegvk.Deployment:             applyDeploymentCustomizer,
			kubegvk.MonitoringStack:        applyObservabilityCustomizer,
			kubegvk.TempoMonolithic:        applyObservabilityCustomizer,
			kubegvk.TempoStack:             applyObservabilityCustomizer,
			kubegvk.OpenTelemetryCollector: applyObservabilityCustomizer,
		},
	}
}

var _ Option = Options{}
