package gc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
)

var (
	ErrRunInputRequired        = errors.New("gc run input is required")
	ErrActionRequired          = errors.New("gc action is required")
	ErrTypeDiscoveryRequired   = errors.New("gc type discovery is required")
	ErrMetadataPolicy          = errors.New("gc metadata policy is required")
	ErrEmptyDiscovery          = errors.New("gc type discovery is empty")
	ErrUnscopedSelector        = errors.New("gc metadata selector must be scoped")
	ErrDiscoveryClientRequired = errors.New("gc discovery client is required")
	ErrDiscoveryMaxAge         = errors.New("gc discovery max age cannot be negative")
	ErrDiscoverySync           = errors.New("gc discovery cache synchronization failed")
)

// Result reports the work performed by one GC run.
type Result struct {
	Discovered int
	Listed     int
	Deleted    int
}

// Action removes stale resources controlled by one owner.
type Action struct {
	discovery TypeDiscovery
	options   Options

	validationErr error
	validated     bool
}

// New creates an immediately usable GC action.
func New(typeDiscovery TypeDiscovery, values ...Option) *Action {
	options := defaultOptions()
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	action := &Action{discovery: typeDiscovery, options: options}
	action.validationErr = action.Validate()
	action.validated = true

	return action
}

// Name returns the default pipeline registration name.
func (a *Action) Name() string {
	return "gc"
}

// Validate checks stable action configuration.
func (a *Action) Validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.validated {
		return a.validationErr
	}
	if isNilInterface(a.discovery) {
		return ErrTypeDiscoveryRequired
	}
	if a.options.MetadataPolicy == nil {
		return ErrMetadataPolicy
	}
	if validator, ok := a.discovery.(discoveryValidator); ok {
		if err := validator.validate(); err != nil {
			return err
		}
	}

	return nil
}

// Run discovers, lists, and deletes stale resources for the supplied owner.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Result, error) {
	runOptions, err := resolveRunOptions(values...)
	if err != nil {
		return Result{}, err
	}

	if err := a.Validate(); err != nil {
		return Result{}, err
	}

	_, err = resources.IdentityOf(runOptions.Owner, runOptions.Client.Scheme())
	if err != nil {
		return Result{}, fmt.Errorf("identify GC owner: %w", err)
	}

	selector := a.options.MetadataPolicy.Selector(runOptions.Owner)
	if selector.String() == "" {
		return Result{}, ErrUnscopedSelector
	}

	desired, err := desiredIdentities(runOptions.Resources, runOptions.Client.Scheme())
	if err != nil {
		return Result{}, err
	}

	types, err := a.discovery.Discover(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("discover GC resource types: %w", err)
	}

	result := Result{Discovered: len(types)}
	if a.options.MetricsEnabled != nil && *a.options.MetricsEnabled {
		GCyclesTotal.Inc()
	}

	for _, gvk := range types {
		deleted, listed, err := a.collectType(
			ctx,
			runOptions,
			selector,
			desired,
			gvk,
		)
		if err != nil {
			return result, err
		}

		result.Listed += listed
		result.Deleted += deleted
	}

	if a.options.MetricsEnabled != nil && *a.options.MetricsEnabled {
		GDeletedTotal.Add(float64(result.Deleted))
	}

	return result, nil
}

func (a *Action) collectType(
	ctx context.Context,
	values RunOptions,
	selector labels.Selector,
	desired map[resources.Identity]struct{},
	gvk schema.GroupVersionKind,
) (int, int, error) {
	if a.isUnremovable(gvk) {
		return 0, 0, nil
	}

	for _, predicate := range a.options.TypePredicates {
		include, err := predicate(ctx, gvk)
		if err != nil {
			return 0, 0, fmt.Errorf("evaluate GC type predicate for %s: %w", gvk, err)
		}
		if !include {
			return 0, 0, nil
		}
	}

	mapping, err := values.Client.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return 0, 0, fmt.Errorf("map GC resource %s: %w", gvk, err)
	}

	allowed, err := canDelete(ctx, values.Client, mapping, gcNamespace(a.options.Namespace, values.Owner, mapping))
	if err != nil {
		return 0, 0, fmt.Errorf("authorize deletion for %s: %w", gvk, err)
	}
	if !allowed {
		return 0, 0, nil
	}

	objects, err := a.list(ctx, values.Client, mapping, selector, values.Owner)
	if err != nil {
		return 0, 0, fmt.Errorf("list GC resources %s: %w", gvk, err)
	}

	deleted := 0
	for index := range objects {
		object := &objects[index]
		object.SetGroupVersionKind(gvk)

		remove, err := a.shouldDelete(ctx, object, values.Owner, desired)
		if err != nil {
			return deleted, len(objects), fmt.Errorf("evaluate GC object %s/%s %s: %w", object.GetNamespace(), object.GetName(), gvk, err)
		}
		if !remove {
			continue
		}

		if err := values.Client.Delete(ctx, object, client.PropagationPolicy(a.options.PropagationPolicy)); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return deleted, len(objects), fmt.Errorf("delete GC object %s/%s %s: %w", object.GetNamespace(), object.GetName(), gvk, err)
		}

		deleted++
	}

	return deleted, len(objects), nil
}

func (a *Action) list(
	ctx context.Context,
	cli client.Client,
	mapping *meta.RESTMapping,
	selector labels.Selector,
	owner client.Object,
) ([]unstructured.Unstructured, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(mapping.GroupVersionKind.GroupVersion().WithKind(mapping.GroupVersionKind.Kind + "List"))

	options := []client.ListOption{client.MatchingLabelsSelector{Selector: selector}}
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		options = append(options, client.InNamespace(gcNamespace(a.options.Namespace, owner, mapping)))
	}

	err := cli.List(ctx, list, options...)
	switch {
	case err == nil:
		return list.Items, nil
	case apierrors.IsForbidden(err):
		return nil, nil
	case apierrors.IsMethodNotSupported(err):
		return nil, nil
	case apierrors.IsNotFound(err):
		return nil, nil
	case meta.IsNoMatchError(err):
		return nil, nil
	case err != nil:
		return nil, err
	}

	return nil, nil
}

func (a *Action) shouldDelete(
	ctx context.Context,
	object *unstructured.Unstructured,
	owner client.Object,
	desired map[resources.Identity]struct{},
) (bool, error) {
	if a.isUnremovable(object.GroupVersionKind()) {
		return false, nil
	}
	if resources.HasAnnotation(object, platformannotations.ManagedByODHOperator) {
		return false, nil
	}
	if !metav1.IsControlledBy(object, owner) {
		return false, nil
	}

	identity := resources.Identity{
		GVK:       object.GroupVersionKind(),
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}
	_, exists := desired[identity]
	if exists && a.options.MetadataPolicy.Matches(object, owner) {
		return false, nil
	}

	for _, predicate := range a.options.ObjectPredicates {
		remove, err := predicate(ctx, object)
		if err != nil {
			return false, err
		}
		if !remove {
			return false, nil
		}
	}

	return true, nil
}

func (a *Action) isUnremovable(gvk schema.GroupVersionKind) bool {
	return slices.Contains(a.options.UnremovableTypes, gvk)
}

func desiredIdentities(
	accessor resources.Accessor,
	scheme *runtime.Scheme,
) (map[resources.Identity]struct{}, error) {
	identities := make(map[resources.Identity]struct{}, accessor.Len())
	for _, object := range accessor.All() {
		identity, err := resources.IdentityOf(object, scheme)
		if err != nil {
			return nil, fmt.Errorf("identify desired GC resource: %w", err)
		}
		identities[identity] = struct{}{}
	}

	return identities, nil
}
