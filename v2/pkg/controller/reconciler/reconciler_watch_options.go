package reconciler

import (
	"context"
	"errors"
	"fmt"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/reconciler/dynamicwatcher"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
	apihelpers "k8s.io/apiextensions-apiserver/pkg/apihelpers"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DynamicPredicate determines whether a conditional watch is applicable for
// the current reconciliation request.
type DynamicPredicate = dynamicwatcher.DynamicPredicate

// WatchOptions configures one secondary watch.
type WatchOptions struct {
	EventHandler      dynamicwatcher.EventHandler
	Predicates        []dynamicwatcher.Predicate
	DynamicPredicates []DynamicPredicate
	Dynamic           bool
}

// ApplyTo applies watch policy while preserving options supplied earlier in a
// builder chain.
func (o WatchOptions) ApplyTo(target *WatchOptions) {
	if o.EventHandler != nil {
		target.EventHandler = o.EventHandler
	}

	target.Predicates = append(target.Predicates, o.Predicates...)
	target.DynamicPredicates = append(target.DynamicPredicates, o.DynamicPredicates...)
	target.Dynamic = target.Dynamic || o.Dynamic
}

// WatchOption configures one secondary watch.
type WatchOption = option.Option[WatchOptions]

var ErrNilWatchPredicate = errors.New("reconciler watch predicate is required")

// WithEventHandler sets the event-to-request mapper for a watch.
func WithEventHandler(value dynamicwatcher.EventHandler) WatchOption {
	return option.FunctionalOption[WatchOptions](func(options *WatchOptions) {
		options.EventHandler = value
	})
}

// WithEventMapper adapts a map function to a controller-runtime event handler.
func WithEventMapper(value dynamicwatcher.MapFunc) WatchOption {
	return option.FunctionalOption[WatchOptions](func(options *WatchOptions) {
		options.EventHandler = dynamicwatcher.EventHandlerFromMap(value)
	})
}

// WithPredicates adds event predicates to a watch.
func WithPredicates(values ...dynamicwatcher.Predicate) WatchOption {
	return option.FunctionalOption[WatchOptions](func(options *WatchOptions) {
		options.Predicates = append(options.Predicates, values...)
	})
}

// Dynamic makes a watch conditional on all supplied predicates.
func Dynamic(values ...DynamicPredicate) WatchOption {
	return option.FunctionalOption[WatchOptions](func(options *WatchOptions) {
		options.DynamicPredicates = append(options.DynamicPredicates, values...)
		options.Dynamic = true
	})
}

// When adapts the same guard used by pipeline actions to a conditional watch.
func When(guard pipeline.Guard) WatchOption {
	if guard == nil {
		return Dynamic(nil)
	}

	return Dynamic(func(ctx context.Context, request *pipeline.Request) (bool, error) {
		return guard.Evaluate(ctx, request)
	})
}

// CrdExists enables a watch when the requested API is available through the
// manager REST mapper. RESTMapper NoMatch is an expected absence result.
func CrdExists(watchedGVK schema.GroupVersionKind) DynamicPredicate {
	return func(ctx context.Context, request *pipeline.Request) (bool, error) {
		if request == nil || request.Client == nil {
			return false, nil
		}

		mapping, err := request.Client.RESTMapper().RESTMapping(
			watchedGVK.GroupKind(),
			watchedGVK.Version,
		)
		switch {
		case meta.IsNoMatchError(err):
			return false, nil
		case err == nil:
			return crdExists(ctx, request, mapping.Resource.GroupResource().String())
		default:
			return false, fmt.Errorf("check API availability for %s: %w", watchedGVK, err)
		}
	}
}

func crdExists(ctx context.Context, request *pipeline.Request, name string) (bool, error) {
	definition := new(apiextensionsv1.CustomResourceDefinition)
	err := request.Client.Get(ctx, client.ObjectKey{Namespace: "", Name: name}, definition)
	switch {
	case err == nil:
		return !apihelpers.IsCRDConditionTrue(definition, apiextensionsv1.Terminating), nil
	case apierrors.IsNotFound(err):
		return false, nil
	case meta.IsNoMatchError(err):
		return false, nil
	default:
		return false, fmt.Errorf("get CRD %q: %w", name, err)
	}
}

// CrdExistsWithoutPreferred enables fallback when the preferred API is not
// available. It avoids registering both versions of one API.
func CrdExistsWithoutPreferred(
	fallbackGVK schema.GroupVersionKind,
	preferredGVK schema.GroupVersionKind,
) DynamicPredicate {
	return func(ctx context.Context, request *pipeline.Request) (bool, error) {
		preferred, err := CrdExists(preferredGVK)(ctx, request)
		if err != nil || preferred {
			return false, err
		}

		return CrdExists(fallbackGVK)(ctx, request)
	}
}

var _ WatchOption = (*WatchOptions)(nil)
