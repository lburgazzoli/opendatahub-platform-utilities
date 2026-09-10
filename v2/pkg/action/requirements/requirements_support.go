package requirements

import (
	"errors"
)

var (
	ErrRequestRequired  = errors.New("requirements pipeline request is required")
	ErrInstanceRequired = errors.New("requirements instance is required")
)
