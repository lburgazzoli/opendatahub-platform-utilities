// Package deployment contains generic Deployment observation actions.
package deployment

import (
	"context"
	"errors"
	"fmt"
	"maps"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

const (
	DefaultConditionType      = "DeploymentsAvailable"
	DefaultNotAvailableReason = "DeploymentsNotReady"
)

var (
	ErrActionRequired        = errors.New("deployment action is required")
	ErrClientRequired        = errors.New("deployment client is required")
	ErrConditionsRequired    = errors.New("deployment conditions accessor is required")
	ErrConditionTypeRequired = errors.New("deployment condition type is required")
	ErrSelectorRequired      = errors.New("deployment selector is required")
	ErrNamespaceRequired     = errors.New("deployment namespace is required")
)

// Observation is the deployment availability result.
type Observation struct {
	Condition api.Condition
	Ready     int
	Total     int
}

// Action observes Deployment availability.
//
//nolint:govet // field order follows the public action policy grouping.
type Action struct {
	labels             map[string]string
	conditionType      string
	notAvailableReason string
	validationErr      error
	validated          bool
}

// New creates a deployment observation action.
func New(values ...Option) *Action {
	options := Options{
		Labels:             make(map[string]string),
		ConditionType:      DefaultConditionType,
		NotAvailableReason: DefaultNotAvailableReason,
	}
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	action := &Action{
		labels:             maps.Clone(options.Labels),
		conditionType:      options.ConditionType,
		notAvailableReason: options.NotAvailableReason,
	}
	action.validationErr = action.Validate()
	action.validated = true

	return action
}

// Validate checks stable action configuration.
func (a *Action) Validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.validated {
		return a.validationErr
	}
	if a.conditionType == "" {
		return ErrConditionTypeRequired
	}
	if len(a.labels) == 0 {
		return ErrSelectorRequired
	}

	return nil
}

// Run observes Deployments in one namespace.
func (a *Action) Run(ctx context.Context, values ...RunOption) (Observation, error) {
	err := a.Validate()
	if err != nil {
		return Observation{}, err
	}

	options, err := resolveRunOptions(values...)
	if err != nil {
		return Observation{}, err
	}
	if options.Namespace == "" {
		return Observation{}, ErrNamespaceRequired
	}

	list := &appsv1.DeploymentList{}
	err = options.Client.List(
		ctx,
		list,
		client.InNamespace(options.Namespace),
		client.MatchingLabelsSelector{Selector: labels.SelectorFromSet(a.labels)},
	)
	if err != nil {
		return Observation{}, fmt.Errorf("list deployments: %w", err)
	}

	ready := 0
	for index := range list.Items {
		deployment := &list.Items[index]
		if deployment.Status.Replicas != 0 && deployment.Status.ReadyReplicas == deployment.Status.Replicas {
			ready++
		}
	}

	status := metav1.ConditionTrue
	reason := "DeploymentsReady"
	if len(list.Items) == 0 || ready != len(list.Items) {
		status = metav1.ConditionFalse
		reason = a.notAvailableReason
	}

	return Observation{
		Condition: api.Condition{
			Type:    a.conditionType,
			Status:  status,
			Reason:  reason,
			Message: fmt.Sprintf("%d/%d deployments ready", ready, len(list.Items)),
		},
		Ready: ready,
		Total: len(list.Items),
	}, nil
}
