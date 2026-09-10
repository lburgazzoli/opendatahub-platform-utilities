package requirements

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/action"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/platform/condition"
)

const (
	// ConditionTypeObjectsAvailable reports required object availability.
	ConditionTypeObjectsAvailable = "ObjectsAvailable"
	// ConditionTypeForbiddenObjectsAbsent reports forbidden object absence.
	ConditionTypeForbiddenObjectsAbsent = "ForbiddenObjectsAbsent"
)

var (
	ErrObjectsRequired   = errors.New("at least one object is required")
	ErrObjectGVKRequired = errors.New("object GVK is required")
)

// ObjectReference identifies an object or all objects of a GVK in an optional
// namespace. An empty Name checks whether any matching object exists.
type ObjectReference struct {
	GVK       schema.GroupVersionKind
	Namespace string
	Name      string
}

type objectMode uint8

const (
	objectMustExist objectMode = iota + 1
	objectMustNotExist
)

// RequireObjectsAction checks that all configured objects exist.
type RequireObjectsAction struct {
	action *objectAction
}

// ForbidObjectsAction checks that all configured objects do not exist.
type ForbidObjectsAction struct {
	action *objectAction
}

// objectAction contains the shared implementation for object requirements.
type objectAction struct {
	name          string
	objects       []ObjectReference
	mode          objectMode
	validationErr error
	validated     bool
}

// RequireObjects creates an action that fails when any requested object is
// absent.
func RequireObjects(objects ...ObjectReference) *RequireObjectsAction {
	return &RequireObjectsAction{
		action: newObjectAction("require-objects", objectMustExist, objects),
	}
}

// ForbidObjects creates an action that fails when any requested object exists.
func ForbidObjects(objects ...ObjectReference) *ForbidObjectsAction {
	return &ForbidObjectsAction{
		action: newObjectAction("forbid-objects", objectMustNotExist, objects),
	}
}

func newObjectAction(
	name string,
	mode objectMode,
	objects []ObjectReference,
) *objectAction {
	action := &objectAction{
		name:    name,
		objects: slices.Clone(objects),
		mode:    mode,
	}
	action.validationErr = action.validate()
	action.validated = true

	return action
}

// Run checks the configured objects against the supplied Kubernetes client.
func (a *RequireObjectsAction) Run(ctx context.Context, values ...RunOption) error {
	if err := a.Validate(); err != nil {
		return err
	}

	return a.action.run(ctx, values...)
}

// Run checks the configured objects against the supplied Kubernetes client.
func (a *ForbidObjectsAction) Run(ctx context.Context, values ...RunOption) error {
	if err := a.Validate(); err != nil {
		return err
	}

	return a.action.run(ctx, values...)
}

// Validate checks stable object requirement configuration.
func (a *RequireObjectsAction) Validate() error {
	if a == nil || a.action == nil {
		return ErrActionRequired
	}

	return a.action.validate()
}

// Validate checks stable object requirement configuration.
func (a *ForbidObjectsAction) Validate() error {
	if a == nil || a.action == nil {
		return ErrActionRequired
	}

	return a.action.validate()
}

func (a *objectAction) validate() error {
	if a == nil {
		return ErrActionRequired
	}
	if a.validated {
		return a.validationErr
	}
	if len(a.objects) == 0 {
		return ErrObjectsRequired
	}

	for _, object := range a.objects {
		if object.GVK.Version == "" || object.GVK.Kind == "" {
			return fmt.Errorf("%w: %s", ErrObjectGVKRequired, formatObjectReference(object))
		}
	}

	return nil
}

func (a *objectAction) run(ctx context.Context, values ...RunOption) error {
	if err := a.validate(); err != nil {
		return err
	}

	inputs, err := resolveRunOptions(values...)
	if err != nil {
		return err
	}

	conditionType, reason := objectCondition(a.mode)
	var unmatched []ObjectReference

	for _, object := range a.objects {
		exists, err := objectExists(ctx, inputs.Client, object)
		if err != nil {
			condition.MarkUnknown(
				inputs.Conditions,
				conditionType,
				condition.WithError(err),
				condition.WithObservedGeneration(inputs.ObservedGeneration),
			)

			return fmt.Errorf("check %s: %w", a.name, err)
		}

		switch a.mode {
		case objectMustExist:
			if !exists {
				unmatched = append(unmatched, object)
			}
		case objectMustNotExist:
			if exists {
				unmatched = append(unmatched, object)
			}
		default:
			return fmt.Errorf("%w: %d", ErrModeInvalid, a.mode)
		}
	}

	if len(unmatched) > 0 {
		condition.MarkFalse(
			inputs.Conditions,
			conditionType,
			condition.WithReason(reason),
			condition.WithMessagef("%s: %s", a.name, strings.Join(formatObjectReferences(unmatched), ", ")),
			condition.WithObservedGeneration(inputs.ObservedGeneration),
		)

		return action.NewErrorf(
			"%s: %s",
			a.name,
			strings.Join(formatObjectReferences(unmatched), ", "),
		).Terminal()
	}

	condition.MarkTrue(
		inputs.Conditions,
		conditionType,
		condition.WithObservedGeneration(inputs.ObservedGeneration),
	)

	return nil
}

func objectCondition(mode objectMode) (string, string) {
	switch mode {
	case objectMustExist:
		return ConditionTypeObjectsAvailable, "ObjectsUnavailable"
	case objectMustNotExist:
		return ConditionTypeForbiddenObjectsAbsent, "ForbiddenObjectsPresent"
	default:
		return "", ""
	}
}

func objectExists(
	ctx context.Context,
	cli client.Client,
	target ObjectReference,
) (bool, error) {
	switch {
	case target.Name != "":
		return objectExistsByName(ctx, cli, target)
	default:
		return anyObjectExists(ctx, cli, target)
	}
}

func objectExistsByName(
	ctx context.Context,
	cli client.Client,
	target ObjectReference,
) (bool, error) {
	object := unstructured.Unstructured{}
	object.SetGroupVersionKind(target.GVK)

	err := cli.Get(ctx, client.ObjectKey{Namespace: target.Namespace, Name: target.Name}, &object)
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	case meta.IsNoMatchError(err):
		return false, nil
	default:
		return false, fmt.Errorf("get %s: %w", formatObjectReference(target), err)
	}
}

func anyObjectExists(
	ctx context.Context,
	cli client.Client,
	target ObjectReference,
) (bool, error) {
	list := unstructured.UnstructuredList{}
	list.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   target.GVK.Group,
		Version: target.GVK.Version,
		Kind:    target.GVK.Kind + "List",
	})

	var options []client.ListOption
	if target.Namespace != "" {
		options = append(options, client.InNamespace(target.Namespace))
	}

	err := cli.List(ctx, &list, options...)
	switch {
	case err == nil:
		return len(list.Items) > 0, nil
	case apierrors.IsNotFound(err):
		return false, nil
	case meta.IsNoMatchError(err):
		return false, nil
	default:
		return false, fmt.Errorf("list %s: %w", formatObjectReference(target), err)
	}
}

func formatObjectReferences(objects []ObjectReference) []string {
	formatted := make([]string, len(objects))
	for index, object := range objects {
		formatted[index] = formatObjectReference(object)
	}

	return formatted
}

func formatObjectReference(object ObjectReference) string {
	if object.Namespace == "" && object.Name == "" {
		return object.GVK.String()
	}

	identity := object.Name
	if identity == "" {
		identity = "*"
	}
	if object.Namespace != "" {
		identity = object.Namespace + "/" + identity
	}

	return object.GVK.String() + " " + identity
}
