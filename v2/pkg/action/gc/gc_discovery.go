package gc

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	extensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/tools/cache"
	crcache "sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

// TypeDiscovery returns API kinds that can be considered by GC.
type TypeDiscovery interface {
	Discover(ctx context.Context) ([]schema.GroupVersionKind, error)
}

// TypeDiscoveryFunc adapts a function to TypeDiscovery.
type TypeDiscoveryFunc func(context.Context) ([]schema.GroupVersionKind, error)

// Discover implements TypeDiscovery.
func (f TypeDiscoveryFunc) Discover(ctx context.Context) ([]schema.GroupVersionKind, error) {
	return f(ctx)
}

// InvalidatableTypeDiscovery can discard a cached discovery snapshot.
type InvalidatableTypeDiscovery interface {
	TypeDiscovery
	Invalidate()
}

type discoveryValidator interface {
	validate() error
}

// DynamicTypeDiscovery discovers listable API kinds from a discovery client.
// The mutex covers setup, the snapshot, refresh, and client-go invalidation so
// an event cannot invalidate a snapshot while a refresh is publishing it.
type DynamicTypeDiscovery struct {
	client  discovery.DiscoveryInterface
	options DynamicDiscoveryOptions

	setupMu   sync.Mutex
	mu        sync.Mutex
	setupDone bool
	setupErr  error
	cached    []schema.GroupVersionKind
	cachedAt  time.Time
	cachedSet bool
}

// DynamicDiscovery creates a dynamic discovery implementation.
func DynamicDiscovery(
	discoveryClient discovery.DiscoveryInterface,
	values ...DynamicDiscoveryOption,
) *DynamicTypeDiscovery {
	options := DynamicDiscoveryOptions{MaxAge: defaultDiscoveryMaxAge}
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}
	return &DynamicTypeDiscovery{client: discoveryClient, options: options}
}

// Discover returns a copy of the current API kind snapshot.
func (d *DynamicTypeDiscovery) Discover(ctx context.Context) ([]schema.GroupVersionKind, error) {
	if d == nil || isNilInterface(d.client) {
		return nil, ErrDiscoveryClientRequired
	}
	if d.options.MaxAge < 0 {
		return nil, ErrDiscoveryMaxAge
	}
	if d.options.EventInvalidationManager == nil {
		return d.discoverLive(ctx)
	}
	return d.discoverCached(ctx)
}

func (d *DynamicTypeDiscovery) discoverCached(ctx context.Context) ([]schema.GroupVersionKind, error) {
	d.setupMu.Lock()
	if !d.setupDone {
		d.setupErr = d.installInvalidation(ctx, d.options.EventInvalidationManager)
		d.setupDone = true
	}
	setupErr := d.setupErr
	d.setupMu.Unlock()
	if setupErr != nil {
		return nil, setupErr
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cachedSet && time.Since(d.cachedAt) < d.options.MaxAge {
		return slices.Clone(d.cached), nil
	}

	result, err := d.discoverLive(ctx)
	if err != nil {
		return nil, err
	}
	d.cached = slices.Clone(result)
	d.cachedAt = time.Now()
	d.cachedSet = true
	return slices.Clone(result), nil
}

// Invalidate discards a cached discovery snapshot and the underlying
// client-go discovery cache while excluding concurrent refreshes.
func (d *DynamicTypeDiscovery) Invalidate() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cached = nil
	d.cachedAt = time.Time{}
	d.cachedSet = false
	if cached, ok := d.client.(discovery.CachedDiscoveryInterface); ok {
		cached.Invalidate()
	}
}

func (d *DynamicTypeDiscovery) discoverLive(context.Context) ([]schema.GroupVersionKind, error) {
	resourceLists, err := d.client.ServerPreferredResources()
	switch {
	case err == nil:
		return discoveredKinds(resourceLists)
	case discovery.IsGroupDiscoveryFailedError(err):
		return discoveredKinds(resourceLists)
	case err != nil:
		return nil, fmt.Errorf("discover API resources: %w", err)
	default:
		return nil, nil
	}
}

func discoveredKinds(resourceLists []*metav1.APIResourceList) ([]schema.GroupVersionKind, error) {
	result := make([]schema.GroupVersionKind, 0)
	for _, resourceList := range resourceLists {
		groupVersion, parseErr := schema.ParseGroupVersion(resourceList.GroupVersion)
		if parseErr != nil {
			return nil, fmt.Errorf("parse discovered group version %q: %w", resourceList.GroupVersion, parseErr)
		}
		for _, resource := range resourceList.APIResources {
			if resource.Name == "" || resource.Kind == "" || strings.Contains(resource.Name, "/") {
				continue
			}
			if !slices.Contains(resource.Verbs, "list") {
				continue
			}
			result = append(result, groupVersion.WithKind(resource.Kind))
		}
	}
	slices.SortFunc(result, func(left schema.GroupVersionKind, right schema.GroupVersionKind) int {
		return strings.Compare(left.String(), right.String())
	})
	return result, nil
}

func (d *DynamicTypeDiscovery) installInvalidation(
	ctx context.Context,
	discoveryManager manager.Manager,
) error {
	crdInformer, err := discoveryManager.GetCache().GetInformer(ctx, &extensionsv1.CustomResourceDefinition{})
	if err != nil {
		return fmt.Errorf("get CRD informer: %w", err)
	}
	if err := addInvalidationHandler(crdInformer, d.Invalidate); err != nil {
		return fmt.Errorf("add CRD invalidation handler: %w", err)
	}

	apiServiceInformer, err := discoveryManager.GetCache().GetInformer(ctx, apiServiceObject())
	switch {
	case err == nil:
		if handlerErr := addInvalidationHandler(apiServiceInformer, d.Invalidate); handlerErr != nil {
			return fmt.Errorf("add APIService invalidation handler: %w", handlerErr)
		}
	case apierrors.IsNotFound(err):
		apiServiceInformer = nil
	case meta.IsNoMatchError(err):
		apiServiceInformer = nil
	case err != nil:
		return fmt.Errorf("get APIService informer: %w", err)
	}

	if !cache.WaitForCacheSync(ctx.Done(), crdInformer.HasSynced) {
		return fmt.Errorf("wait for CRD informer synchronization: %w", synchronizationError(ctx))
	}
	if apiServiceInformer != nil && !cache.WaitForCacheSync(ctx.Done(), apiServiceInformer.HasSynced) {
		return fmt.Errorf("wait for APIService informer synchronization: %w", synchronizationError(ctx))
	}
	d.Invalidate()
	return nil
}

func addInvalidationHandler(informer crcache.Informer, invalidate func()) error {
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    func(any) { invalidate() },
		UpdateFunc: func(any, any) { invalidate() },
		DeleteFunc: func(any) { invalidate() },
	})
	return err
}

func synchronizationError(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrDiscoverySync
}

func apiServiceObject() client.Object {
	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiregistration.k8s.io",
		Version: "v1",
		Kind:    "APIService",
	})
	return object
}

// StaticDiscovery returns an immutable finite discovery set.
func StaticDiscovery(values ...schema.GroupVersionKind) TypeDiscovery {
	return &staticTypeDiscovery{types: slices.Clone(values)}
}

type staticTypeDiscovery struct {
	types []schema.GroupVersionKind
}

func (d *staticTypeDiscovery) Discover(context.Context) ([]schema.GroupVersionKind, error) {
	return slices.Clone(d.types), nil
}

func (d *staticTypeDiscovery) validate() error {
	if len(d.types) == 0 {
		return ErrEmptyDiscovery
	}
	return nil
}

var _ InvalidatableTypeDiscovery = (*DynamicTypeDiscovery)(nil)
var _ TypeDiscovery = (*staticTypeDiscovery)(nil)
var _ discoveryValidator = (*staticTypeDiscovery)(nil)
