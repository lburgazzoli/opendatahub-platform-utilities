package gc

import (
	"time"

	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/option"
)

const defaultDiscoveryMaxAge = 5 * time.Minute

// DynamicDiscoveryOptions configures optional event-backed discovery caching.
type DynamicDiscoveryOptions struct {
	EventInvalidationManager manager.Manager
	MaxAge                   time.Duration
}

// ApplyTo copies explicitly supplied discovery options.
func (o DynamicDiscoveryOptions) ApplyTo(target *DynamicDiscoveryOptions) {
	if o.EventInvalidationManager != nil {
		target.EventInvalidationManager = o.EventInvalidationManager
	}
	if o.MaxAge != 0 {
		target.MaxAge = o.MaxAge
	}
}

// DynamicDiscoveryOption configures dynamic discovery.
type DynamicDiscoveryOption = option.Option[DynamicDiscoveryOptions]

// WithEventInvalidation enables manager-backed discovery invalidation.
func WithEventInvalidation(manager manager.Manager) DynamicDiscoveryOption {
	return option.FunctionalOption[DynamicDiscoveryOptions](func(options *DynamicDiscoveryOptions) {
		if manager != nil {
			options.EventInvalidationManager = manager
		}
	})
}

// WithDiscoveryMaxAge bounds the age of an event-backed discovery snapshot.
func WithDiscoveryMaxAge(maxAge time.Duration) DynamicDiscoveryOption {
	return option.FunctionalOption[DynamicDiscoveryOptions](func(options *DynamicDiscoveryOptions) {
		options.MaxAge = maxAge
	})
}

var _ DynamicDiscoveryOption = DynamicDiscoveryOptions{}
