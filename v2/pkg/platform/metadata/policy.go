package metadata

import (
	"errors"
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	platformannotations "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/annotations"
	platformlabels "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/metadata/labels"
)

var (
	ErrObjectRequired = errors.New("metadata object is required")
	ErrOwnerRequired  = errors.New("metadata owner is required")
	ErrOwnerIdentity  = errors.New("metadata owner identity is incomplete")
	ErrOwnerGVK       = errors.New("metadata owner GVK is required")
	ErrPolicyMismatch = errors.New("metadata policy produced non-matching metadata")
)

type Policy interface {
	Apply(object client.Object, owner client.Object) error
	Selector(owner client.Object) labels.Selector
	Matches(object client.Object, owner client.Object) bool
}

type defaultPolicy struct{}

func DefaultPolicy() Policy { return defaultPolicy{} }

func (defaultPolicy) Apply(object client.Object, owner client.Object) error {
	if object == nil {
		return ErrObjectRequired
	}

	values, err := ownerValues(owner)
	if err != nil {
		return err
	}

	resources.SetLabels(object, map[string]string{
		platformlabels.PlatformPartOf: values.partOf,
	})

	resources.SetAnnotations(object, map[string]string{
		platformannotations.InstanceName:       values.name,
		platformannotations.InstanceNamespace:  values.namespace,
		platformannotations.InstanceUID:        values.uid,
		platformannotations.InstanceGeneration: values.generation,
	})

	return nil
}

func (defaultPolicy) Selector(owner client.Object) labels.Selector {
	values, err := ownerValues(owner)
	if err != nil {
		return labels.Nothing()
	}

	return labels.SelectorFromSet(labels.Set{platformlabels.PlatformPartOf: values.partOf})
}

func (policy defaultPolicy) Matches(object client.Object, owner client.Object) bool {
	if object == nil {
		return false
	}

	values, err := ownerValues(owner)
	if err != nil {
		return false
	}

	return policy.Selector(owner).Matches(labels.Set(object.GetLabels())) &&
		object.GetAnnotations()[platformannotations.InstanceName] == values.name &&
		object.GetAnnotations()[platformannotations.InstanceNamespace] == values.namespace &&
		object.GetAnnotations()[platformannotations.InstanceUID] == values.uid &&
		object.GetAnnotations()[platformannotations.InstanceGeneration] == values.generation
}

type composedPolicy struct{ policies []Policy }

func Compose(first Policy, rest ...Policy) Policy {
	policies := make([]Policy, 1, len(rest)+1)
	policies[0] = first
	policies = append(policies, rest...)

	return composedPolicy{policies: policies}
}

func (policy composedPolicy) Apply(object client.Object, owner client.Object) error {
	for index, current := range policy.policies {
		if current == nil {
			return fmt.Errorf("metadata policy %d: %w", index, ErrOwnerRequired)
		}

		err := current.Apply(object, owner)
		if err != nil {
			return fmt.Errorf("metadata policy %d: %w", index, err)
		}
	}

	for index, current := range policy.policies {
		if !current.Matches(object, owner) {
			return fmt.Errorf("%w: policy %d", ErrPolicyMismatch, index)
		}
	}

	return nil
}

func (policy composedPolicy) Matches(object client.Object, owner client.Object) bool {
	for _, current := range policy.policies {
		if current == nil || !current.Matches(object, owner) {
			return false
		}
	}

	return true
}

func (policy composedPolicy) Selector(owner client.Object) labels.Selector {
	selector := labels.NewSelector()

	for _, current := range policy.policies {
		if current == nil {
			return labels.Nothing()
		}

		requirements, selectable := current.Selector(owner).Requirements()
		if !selectable {
			return labels.Nothing()
		}

		selector = selector.Add(requirements...)
	}

	return selector
}

type ownerMetadata struct {
	partOf     string
	name       string
	namespace  string
	uid        string
	generation string
}

func ownerValues(owner client.Object) (ownerMetadata, error) {
	if owner == nil {
		return ownerMetadata{}, ErrOwnerRequired
	}

	if owner.GetName() == "" || owner.GetUID() == "" {
		return ownerMetadata{}, ErrOwnerIdentity
	}

	kind := owner.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		return ownerMetadata{}, ErrOwnerGVK
	}

	partOf, err := platformlabels.NormalizePartOfValue(kind)
	if err != nil {
		return ownerMetadata{}, fmt.Errorf("normalize owner kind: %w", err)
	}

	return ownerMetadata{
		partOf:     partOf,
		name:       owner.GetName(),
		namespace:  owner.GetNamespace(),
		uid:        string(owner.GetUID()),
		generation: strconv.FormatInt(owner.GetGeneration(), 10),
	}, nil
}
