// Package imagestream contains OpenShift ImageStream observation actions.
package imagestream

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

//nolint:gochecknoglobals // the GVK is an immutable API identity.
var imageStreamGVK = schema.GroupVersionKind{
	Group: "image.openshift.io", Version: "v1", Kind: "ImageStream",
}

const (
	DefaultConditionType      = "ImageStreamsAvailable"
	DefaultNotAvailableReason = "ImageStreamsNotReady"
	maxConditionMessageLen    = 100
	maxFailedTags             = 10
)

var (
	ErrActionRequired        = errors.New("imagestream action is required")
	ErrClientRequired        = errors.New("imagestream client is required")
	ErrConditionsRequired    = errors.New("imagestream conditions accessor is required")
	ErrConditionTypeRequired = errors.New("imagestream condition type is required")
	ErrSelectorRequired      = errors.New("imagestream selector is required")
	ErrNamespaceRequired     = errors.New("imagestream namespace is required")
	ErrMalformedStatus       = errors.New("malformed ImageStream status")
)

// Observation reports failed ImageStream imports.
type Observation struct {
	Condition api.Condition
	Failed    int
}

// Action observes OpenShift ImageStream import status.
//
//nolint:govet // field order follows the public action policy grouping.
type Action struct {
	labels             map[string]string
	conditionType      string
	notAvailableReason string
	validationErr      error
	validated          bool
}

// New creates an ImageStream observation action.
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

// Run observes ImageStreams in one namespace.
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

	list, unavailable, err := listImageStreams(ctx, options.Client, options.Namespace, a.labels)
	if err != nil {
		return Observation{}, err
	}
	if unavailable {
		return healthyObservation(a.conditionType), nil
	}

	failedCount, reported, err := failedImageTags(list)
	if err != nil {
		return Observation{}, err
	}

	if failedCount == 0 {
		return healthyObservation(a.conditionType), nil
	}

	suffix := ""
	if failedCount > len(reported) {
		suffix = fmt.Sprintf("; ... and %d more", failedCount-len(reported))
	}

	return Observation{
		Condition: api.Condition{
			Type:   a.conditionType,
			Status: metav1.ConditionFalse,
			Reason: a.notAvailableReason,
			Message: fmt.Sprintf(
				"Warning: %d ImageStream tag(s) failed to import: %s%s",
				failedCount, strings.Join(reported, "; "), suffix,
			),
		},
		Failed: failedCount,
	}, nil
}
