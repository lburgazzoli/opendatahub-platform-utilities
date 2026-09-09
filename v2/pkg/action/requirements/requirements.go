// Package requirements contains actions that gate reconciliation on cluster state.
package requirements

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/cluster"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

const (
	ConditionTypeDependenciesAvailable = "DependenciesAvailable"
	ConditionTypeForbiddenAPIsAbsent   = "ForbiddenAPIsAbsent"
)

var (
	ErrActionRequired     = errors.New("requirements action is required")
	ErrClientRequired     = errors.New("requirements client is required")
	ErrConditionsRequired = errors.New("requirements conditions accessor is required")
	ErrAPIsRequired       = errors.New("at least one API is required")
	ErrModeInvalid        = errors.New("requirements API mode is invalid")
)

type apiMode uint8

const (
	apiMustExist apiMode = iota + 1
	apiMustNotExist
)

//nolint:govet // field order keeps the action identity and cached validation together.
type apiAction struct {
	name          string
	gvks          []schema.GroupVersionKind
	mode          apiMode
	validationErr error
	validated     bool
}

// RequireAPIs creates an action that fails when any requested API is absent.
func RequireAPIs(gvks ...schema.GroupVersionKind) *RequireAPIsAction {
	return &RequireAPIsAction{action: newAPIAction("require-apis", apiMustExist, gvks)}
}

// ForbidAPIs creates an action that fails when any requested API is available.
func ForbidAPIs(gvks ...schema.GroupVersionKind) *ForbidAPIsAction {
	return &ForbidAPIsAction{action: newAPIAction("forbid-apis", apiMustNotExist, gvks)}
}

// RequireAPIsAction checks that all configured APIs are available.
type RequireAPIsAction struct{ action *apiAction }

// ForbidAPIsAction checks that all configured APIs are unavailable.
type ForbidAPIsAction struct{ action *apiAction }

func newAPIAction(name string, mode apiMode, gvks []schema.GroupVersionKind) *apiAction {
	action := &apiAction{name: name, gvks: slices.Clone(gvks), mode: mode}
	action.validationErr = action.validate()
	action.validated = true

	return action
}

func (a *RequireAPIsAction) Run(ctx context.Context, values ...RunOption) error {
	err := a.Validate()
	if err != nil {
		return err
	}

	return a.action.run(values...)
}

func (a *ForbidAPIsAction) Run(ctx context.Context, values ...RunOption) error {
	err := a.Validate()
	if err != nil {
		return err
	}

	return a.action.run(values...)
}

func (a *RequireAPIsAction) Validate() error {
	if a == nil || a.action == nil {
		return ErrActionRequired
	}

	return a.action.validate()
}

func (a *ForbidAPIsAction) Validate() error {
	if a == nil || a.action == nil {
		return ErrActionRequired
	}

	return a.action.validate()
}

func (a *apiAction) validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.validated {
		return a.validationErr
	}
	if len(a.gvks) == 0 {
		return ErrAPIsRequired
	}

	return nil
}

func (a *apiAction) run(values ...RunOption) error {
	err := a.validate()
	if err != nil {
		return err
	}
	inputs, err := resolveRunOptions(values...)
	if err != nil {
		return err
	}

	var conditionType string
	var reason string
	switch a.mode {
	case apiMustExist:
		conditionType = ConditionTypeDependenciesAvailable
		reason = "APIsUnavailable"
	case apiMustNotExist:
		conditionType = ConditionTypeForbiddenAPIsAbsent
		reason = "ForbiddenAPIsAvailable"
	default:
		return fmt.Errorf("%w: %d", ErrModeInvalid, a.mode)
	}

	available, err := availableAPIs(inputs.Client, a.gvks)
	if err != nil {
		condition.MarkUnknown(
			inputs.Conditions,
			conditionType,
			condition.WithError(err),
			condition.WithObservedGeneration(inputs.ObservedGeneration),
		)
		return fmt.Errorf("check %s: %w", a.name, err)
	}

	var matched []schema.GroupVersionKind
	switch a.mode {
	case apiMustExist:
		matched = missingAPIs(a.gvks, available)
	case apiMustNotExist:
		matched = presentAPIs(a.gvks, available)
	}

	if len(matched) > 0 {
		condition.MarkFalse(
			inputs.Conditions,
			conditionType,
			condition.WithReason(reason),
			condition.WithMessage("%s: %s", a.name, strings.Join(formatGVKs(matched), ", ")),
			condition.WithObservedGeneration(inputs.ObservedGeneration),
		)
		return action.NewErrorf("%s: %s", a.name, strings.Join(formatGVKs(matched), ", ")).Terminal()
	}

	condition.MarkTrue(
		inputs.Conditions,
		conditionType,
		condition.WithObservedGeneration(inputs.ObservedGeneration),
	)

	return nil
}

func availableAPIs(
	cli client.Client,
	gvks []schema.GroupVersionKind,
) (map[schema.GroupVersionKind]struct{}, error) {
	available := make(map[schema.GroupVersionKind]struct{}, len(gvks))
	for _, gvk := range gvks {
		hasAPI, err := cluster.HasAPI(cli, gvk)
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", gvk, err)
		}

		if hasAPI {
			available[gvk] = struct{}{}
		}
	}

	return available, nil
}

func missingAPIs(
	gvks []schema.GroupVersionKind,
	available map[schema.GroupVersionKind]struct{},
) []schema.GroupVersionKind {
	missing := make([]schema.GroupVersionKind, 0)
	for _, gvk := range gvks {
		if _, found := available[gvk]; !found {
			missing = append(missing, gvk)
		}
	}

	return missing
}

func presentAPIs(
	gvks []schema.GroupVersionKind,
	available map[schema.GroupVersionKind]struct{},
) []schema.GroupVersionKind {
	present := make([]schema.GroupVersionKind, 0)
	for _, gvk := range gvks {
		if _, found := available[gvk]; found {
			present = append(present, gvk)
		}
	}

	return present
}

func formatGVKs(gvks []schema.GroupVersionKind) []string {
	formatted := make([]string, len(gvks))
	for index, gvk := range gvks {
		formatted[index] = gvk.String()
	}

	return formatted
}
