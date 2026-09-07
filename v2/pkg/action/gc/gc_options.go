package gc

import (
	"context"
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	kubegvk "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/gvk"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	platformmetadata "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata"
)

// TypePredicate decides whether a discovered kind is scanned.
type TypePredicate func(context.Context, schema.GroupVersionKind) (bool, error)

// ObjectPredicate decides whether a listed object is eligible for deletion.
type ObjectPredicate func(context.Context, *unstructured.Unstructured) (bool, error)

// Options configures the GC policy. Non-nil slices and pointers replace the
// current value when applied and are copied before being retained.
//
//nolint:govet // field grouping mirrors the public configuration contract.
type Options struct {
	MetadataPolicy    platformmetadata.Policy
	PropagationPolicy metav1.DeletionPropagation
	TypePredicates    []TypePredicate
	ObjectPredicates  []ObjectPredicate
	UnremovableTypes  []schema.GroupVersionKind
	Namespace         *string
	MetricsEnabled    *bool
}

// ApplyTo applies a complete GC configuration.
func (o Options) ApplyTo(target *Options) {
	if o.MetadataPolicy != nil {
		target.MetadataPolicy = o.MetadataPolicy
	}
	if o.PropagationPolicy != "" {
		target.PropagationPolicy = o.PropagationPolicy
	}
	if o.TypePredicates != nil {
		target.TypePredicates = slices.Clone(o.TypePredicates)
	}
	if o.ObjectPredicates != nil {
		target.ObjectPredicates = slices.Clone(o.ObjectPredicates)
	}
	if o.UnremovableTypes != nil {
		target.UnremovableTypes = slices.Clone(o.UnremovableTypes)
	}
	if o.Namespace != nil {
		target.Namespace = new(*o.Namespace)
	}
	if o.MetricsEnabled != nil {
		target.MetricsEnabled = new(*o.MetricsEnabled)
	}
}

// Option configures an Action.
type Option = option.Option[Options]

// WithMetadataPolicy replaces the metadata policy. Nil leaves the default.
func WithMetadataPolicy(policy platformmetadata.Policy) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if policy != nil {
			options.MetadataPolicy = policy
		}
	})
}

// WithPropagationPolicy sets the deletion propagation policy.
func WithPropagationPolicy(policy metav1.DeletionPropagation) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if policy != "" {
			options.PropagationPolicy = policy
		}
	})
}

// WithTypePredicate appends a type predicate.
func WithTypePredicate(predicate TypePredicate) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if predicate != nil {
			options.TypePredicates = append(options.TypePredicates, predicate)
		}
	})
}

// WithObjectPredicate appends an object predicate.
func WithObjectPredicate(predicate ObjectPredicate) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		if predicate != nil {
			options.ObjectPredicates = append(options.ObjectPredicates, predicate)
		}
	})
}

// WithUnremovableTypes appends GVKs that GC must never delete.
func WithUnremovableTypes(values ...schema.GroupVersionKind) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.UnremovableTypes = append(options.UnremovableTypes, values...)
	})
}

// WithNamespace restricts namespaced listing and authorization to namespace.
func WithNamespace(namespace string) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.Namespace = new(namespace)
	})
}

// WithMetrics controls metrics recording.
func WithMetrics(enabled bool) Option {
	return option.FunctionalOption[Options](func(options *Options) {
		options.MetricsEnabled = new(enabled)
	})
}

func defaultOptions() Options {
	return Options{
		MetadataPolicy:    platformmetadata.DefaultPolicy(),
		PropagationPolicy: metav1.DeletePropagationForeground,
		UnremovableTypes: []schema.GroupVersionKind{
			kubegvk.CustomResourceDefinition,
			kubegvk.Lease,
		},
	}
}

var _ Option = Options{}
