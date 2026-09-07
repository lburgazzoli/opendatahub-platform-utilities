package validation

import (
	"errors"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

var (
	ErrNilObject  = errors.New("platform object is required")
	ErrNilStatus  = errors.New("status accessor returned nil")
	ErrNilRelease = errors.New("release accessor returned nil")
	ErrRoundTrip  = errors.New("optional accessor did not round-trip")
)

func Validate(object api.PlatformObject) error {
	if object == nil || isNil(object) {
		return ErrNilObject
	}

	return errors.Join(validateRequired(object), validateOptional(object))
}

func validateRequired(object api.PlatformObject) error {
	var validationErrors []error
	if object.GetStatus() == nil {
		validationErrors = append(validationErrors, ErrNilStatus)
	}

	if object.GetReleaseStatus() == nil {
		validationErrors = append(validationErrors, ErrNilRelease)
	}

	return errors.Join(validationErrors...)
}

func validateOptional(object api.PlatformObject) error {
	var validationErrors []error

	if accessor, ok := object.(api.ConditionsAccessor); ok {
		err := validateConditions(accessor)
		if err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	if accessor, ok := object.(api.PhaseStatusAccessor); ok {
		err := validatePhase(accessor)
		if err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	if accessor, ok := object.(api.PlatformProfileAccessor); ok {
		err := validateProfile(accessor)
		if err != nil {
			validationErrors = append(validationErrors, err)
		}
	}

	return errors.Join(validationErrors...)
}

func isNil(value any) bool {
	reflected := reflect.ValueOf(value)
	return reflected.Kind() == reflect.Pointer && reflected.IsNil()
}

func validateConditions(accessor api.ConditionsAccessor) error {
	original := accessor.GetConditions()
	defer accessor.SetConditions(original)

	value := api.Condition{Type: "validation", Status: metav1.ConditionTrue}
	accessor.SetConditions([]api.Condition{value})

	if got := accessor.GetConditions(); len(got) != 1 || got[0].Type != value.Type {
		return fmt.Errorf("%w: conditions", ErrRoundTrip)
	}

	return nil
}

func validatePhase(accessor api.PhaseStatusAccessor) error {
	original := accessor.GetPhaseStatus()
	if original == nil {
		return fmt.Errorf("%w: phase pointer is nil", ErrRoundTrip)
	}
	defer accessor.SetPhaseStatus(*original)

	accessor.SetPhaseStatus(api.PhaseStatus{Phase: api.PhaseReady})

	if got := accessor.GetPhaseStatus(); got == nil || got.Phase != api.PhaseReady {
		return fmt.Errorf("%w: phase", ErrRoundTrip)
	}

	return nil
}

func validateProfile(accessor api.PlatformProfileAccessor) error {
	original := accessor.GetPlatformProfile()
	if original == nil {
		return fmt.Errorf("%w: profile pointer is nil", ErrRoundTrip)
	}
	defer accessor.SetPlatformProfile(*original)

	value := api.PlatformProfile{Kind: "validation"}
	accessor.SetPlatformProfile(value)

	if got := accessor.GetPlatformProfile(); got == nil || got.Kind != value.Kind {
		return fmt.Errorf("%w: profile", ErrRoundTrip)
	}

	return nil
}
