package labels

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const PlatformPartOf = "platform.opendatahub.io/part-of"

var (
	ErrValueTooLong = errors.New("metadata label value exceeds 63 characters")
	ErrValueInvalid = errors.New("metadata label value is invalid")
	valuePattern    = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)
)

func NormalizePartOfValue(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return value, nil
	}

	if len(value) > 63 {
		return "", fmt.Errorf("%w: %q", ErrValueTooLong, value)
	}

	if !valuePattern.MatchString(value) {
		return "", fmt.Errorf("%w: %q", ErrValueInvalid, value)
	}

	return value, nil
}
