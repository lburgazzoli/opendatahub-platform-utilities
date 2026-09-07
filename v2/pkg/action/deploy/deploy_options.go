package deploy

import (
	"maps"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	platformmetadata "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata"
)

// Mode selects the Kubernetes write mechanism.
type Mode uint8

const (
	ModeUnspecified Mode = iota
	ModeSSA
	ModePatch
)

// CacheOptions configures the optional process-local deploy cache.
type CacheOptions struct {
	TTL time.Duration
}

// Options configures an Action. Maps and slices are copied when options are
// applied, and a non-nil collection replaces the current collection.
//
//nolint:govet // field order follows the public option grouping and readability.
type Options struct {
	Mode                 Mode
	ContinueOnError      bool
	MetadataPolicy       platformmetadata.Policy
	FieldOwner           FieldOwnerFunc
	Labels               map[string]string
	Annotations          map[string]string
	Sort                 SortFunc
	Cache                *CacheOptions
	MergeStrategies      map[schema.GroupVersionKind]MergeFunc
	ExcludeFromOwnership []schema.GroupVersionKind
	ManagedAnnotation    string
	LegacyOwners         []schema.GroupVersionKind
	ApplyCustomizers     map[schema.GroupVersionKind]CustomizerFunc
	PatchCustomizers     map[schema.GroupVersionKind]CustomizerFunc
}

// ApplyTo applies a complete options value over the current configuration.
//
//nolint:cyclop // each option field has independent documented zero semantics.
func (o Options) ApplyTo(target *Options) {
	if o.Mode != ModeUnspecified {
		target.Mode = o.Mode
	}
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
	if o.MergeStrategies != nil {
		target.MergeStrategies = maps.Clone(o.MergeStrategies)
	}
	if o.ExcludeFromOwnership != nil {
		target.ExcludeFromOwnership = append([]schema.GroupVersionKind(nil), o.ExcludeFromOwnership...)
	}
	target.ContinueOnError = o.ContinueOnError
	if o.ManagedAnnotation != "" {
		target.ManagedAnnotation = o.ManagedAnnotation
	}
	if o.LegacyOwners != nil {
		target.LegacyOwners = append([]schema.GroupVersionKind(nil), o.LegacyOwners...)
	}
	if o.ApplyCustomizers != nil {
		target.ApplyCustomizers = maps.Clone(o.ApplyCustomizers)
	}
	if o.PatchCustomizers != nil {
		target.PatchCustomizers = maps.Clone(o.PatchCustomizers)
	}
}

// Option configures an Action.
type Option = option.Option[Options]

// WithMode selects SSA or patch mode.
func WithMode(mode Mode) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Mode = mode
	})
}

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

// WithSort replaces the apply-order strategy.
func WithSort(sort SortFunc) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Sort = sort
	})
}

// WithApplyOrder installs the default dependency ordering.
func WithApplyOrder() Option { return WithSort(defaultApplyOrder) }

// WithCache enables deploy caching. No argument uses the default TTL.
func WithCache(values ...*CacheOptions) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		cache := CacheOptions{}
		if len(values) > 0 && values[0] != nil {
			cache = *values[0]
		}
		options.Cache = &cache
	})
}

// WithMergeStrategy registers a merge strategy for one GVK.
func WithMergeStrategy(gvk schema.GroupVersionKind, merge MergeFunc) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.MergeStrategies == nil {
			options.MergeStrategies = make(map[schema.GroupVersionKind]MergeFunc)
		}
		options.MergeStrategies[gvk] = merge
	})
}

// WithContinueOnError controls whether deployment continues after an
// individual resource fails.
func WithContinueOnError(enabled bool) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ContinueOnError = enabled
	})
}

// WithManagedAnnotation changes the managed-resource opt-out annotation.
func WithManagedAnnotation(key string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ManagedAnnotation = key
	})
}

// WithExcludeFromOwnership adds GVKs that must not receive owner references.
func WithExcludeFromOwnership(gvks ...schema.GroupVersionKind) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.ExcludeFromOwnership = append(options.ExcludeFromOwnership, gvks...)
	})
}

// WithLegacyOwners permits replacement of matching legacy owner references.
func WithLegacyOwners(gvks ...schema.GroupVersionKind) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.LegacyOwners = append(options.LegacyOwners, gvks...)
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

// WithPatchCustomizer registers a patch customizer for one GVK.
func WithPatchCustomizer(gvk schema.GroupVersionKind, customizer CustomizerFunc) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if options.PatchCustomizers == nil {
			options.PatchCustomizers = make(map[schema.GroupVersionKind]CustomizerFunc)
		}
		options.PatchCustomizers[gvk] = customizer
	})
}

func defaultOptions() Options {
	return Options{
		Mode:           ModeSSA,
		MetadataPolicy: platformmetadata.DefaultPolicy(),
		FieldOwner: func(owner client.Object) string {
			return strings.ToLower(owner.GetObjectKind().GroupVersionKind().Kind)
		},
		Sort:              defaultApplyOrder,
		ManagedAnnotation: "opendatahub.io/managed",
		ExcludeFromOwnership: []schema.GroupVersionKind{{
			Version: "v1",
			Kind:    kindNamespace,
		}},
		MergeStrategies: map[schema.GroupVersionKind]MergeFunc{
			appsv1.SchemeGroupVersion.WithKind("Deployment"): MergeDeployments,
		},
		ApplyCustomizers: map[schema.GroupVersionKind]CustomizerFunc{
			{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: kindClusterRole}: applyClusterRoleCustomizer,
		},
		PatchCustomizers: map[schema.GroupVersionKind]CustomizerFunc{
			appsv1.SchemeGroupVersion.WithKind("Deployment"): patchDeploymentCustomizer,
		},
	}
}

var _ Option = Options{}
