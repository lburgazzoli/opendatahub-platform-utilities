// Package deploy applies a desired Kubernetes resource collection.
package deploy

import (
	"context"
	"errors"
	"fmt"
	"maps"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

var (
	ErrRunInputRequired  = errors.New("deploy run input is required")
	ErrDuplicateIdentity = errors.New("duplicate resource identity")
	ErrUnsupportedMode   = errors.New("unsupported deploy mode")
	ErrActionRequired    = errors.New("deploy action is required")
	ErrMetadataPolicy    = errors.New("deploy metadata policy is required")
	ErrMergeStrategyNil  = errors.New("deploy merge strategy is nil")
	ErrFieldOwner        = errors.New("deploy field owner is required")
)

// MergeFunc preserves caller-controlled fields while preparing a desired
// object for deployment.
type MergeFunc func(
	existing *unstructured.Unstructured,
	desired *unstructured.Unstructured,
) error

// CustomizerFunc can modify an object immediately before it is applied or
// patched. The live object is nil when it does not exist.
type CustomizerFunc func(
	ctx context.Context,
	kubernetesClient client.Client,
	action *Action,
	desired *unstructured.Unstructured,
	existing *unstructured.Unstructured,
) error

// FieldOwnerFunc resolves the SSA field manager for a run owner.
type FieldOwnerFunc func(client.Object) string

// Result reports the resource identities applied and skipped by a run.
type Result struct {
	Applied []resources.Identity
	Skipped []resources.Identity
}

// Action deploys resources using an immutable configuration snapshot.
type Action struct {
	options Options
	cache   *Cache
}

// New creates an immediately usable deploy action.
func New(values ...Option) *Action {
	options := defaultOptions()
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	return &Action{
		options: options,
		cache:   newCache(options.Cache),
	}
}

// Name returns the default pipeline registration name.
func (a *Action) Name() string { return "deploy" }

// Validate checks stable action configuration.
func (a *Action) Validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.options.MetadataPolicy == nil {
		return ErrMetadataPolicy
	}
	if a.options.FieldOwner == nil {
		return ErrFieldOwner
	}
	if a.options.Mode != ModeSSA && a.options.Mode != ModePatch {
		return fmt.Errorf("%w: %d", ErrUnsupportedMode, a.options.Mode)
	}
	for gvk, merge := range a.options.MergeStrategies {
		if merge == nil {
			return fmt.Errorf("%w: %s", ErrMergeStrategyNil, gvk)
		}
	}
	return nil
}

// Run normalizes, decorates, and applies the desired resources.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Result, error) {
	err := a.Validate()
	if err != nil {
		return Result{}, err
	}
	runOptions, err := resolveRunOptions(values...)
	if err != nil {
		return Result{}, err
	}

	objects, err := a.prepare(ctx, runOptions)
	if err != nil {
		return Result{}, err
	}

	result := Result{
		Applied: make([]resources.Identity, 0, len(objects)),
		Skipped: make([]resources.Identity, 0),
	}
	var runErrors []error
	for _, object := range objects {
		identity, err := resources.IdentityOf(object, runOptions.Client.Scheme())
		if err != nil {
			return result, err
		}
		applied, err := a.deployOne(ctx, runOptions, object)
		if err != nil {
			wrapped := fmt.Errorf("deploy %s: %w", identity.GVK, err)
			if !a.options.ContinueOnError {
				return result, wrapped
			}
			runErrors = append(runErrors, wrapped)
			continue
		}
		if applied {
			result.Applied = append(result.Applied, identity)
		} else {
			result.Skipped = append(result.Skipped, identity)
		}
	}

	return result, errors.Join(runErrors...)
}

func (a *Action) prepare(ctx context.Context, values RunOptions) ([]client.Object, error) {
	objects := values.Resources.Get()
	seen := sets.New[resources.Identity]()
	for index, object := range objects {
		identity, err := resources.IdentityOf(object, values.Client.Scheme())
		if err != nil {
			return nil, fmt.Errorf("normalize resource %d: %w", index, err)
		}
		if seen.Has(identity) {
			return nil, fmt.Errorf("%w: %s/%s %s", ErrDuplicateIdentity, identity.Namespace, identity.Name, identity.GVK)
		}
		seen.Insert(identity)
		err = a.decorate(object, values.Owner)
		if err != nil {
			return nil, fmt.Errorf("decorate %s/%s: %w", object.GetNamespace(), object.GetName(), err)
		}
	}

	if a.options.Sort != nil {
		sorted, err := a.options.Sort(ctx, objects)
		if err != nil {
			return nil, fmt.Errorf("sort resources: %w", err)
		}
		objects = sorted
	}
	values.Resources.Set(objects)
	if a.cache != nil {
		a.cache.Sync()
	}

	return objects, nil
}

func (a *Action) decorate(object client.Object, owner client.Object) error {
	object.SetLabels(mergeStringMap(object.GetLabels(), a.options.Labels))
	object.SetAnnotations(mergeStringMap(object.GetAnnotations(), a.options.Annotations))
	return a.options.MetadataPolicy.Apply(object, owner)
}

func mergeStringMap(current map[string]string, additions map[string]string) map[string]string {
	merged := maps.Clone(current)
	if merged == nil {
		merged = make(map[string]string)
	}
	maps.Copy(merged, additions)
	return merged
}
