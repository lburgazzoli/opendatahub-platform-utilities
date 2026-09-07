// Package release contains component release metadata discovery actions.
package release

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"gopkg.in/yaml.v3"

	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
)

const ComponentMetadataFilename = "component_metadata.yaml"

var (
	ErrActionRequired        = errors.New("release action is required")
	ErrReleaseStatusRequired = errors.New("release status accessor is required")
	ErrFilesystemRequired    = errors.New("release filesystem is required")
	ErrPathRequired          = errors.New("release metadata path is required")
)

// Action reads component release metadata from a filesystem.
//
//nolint:govet // field order follows the action's stable policy grouping.
type Action struct {
	fsys          fs.FS
	path          string
	validationErr error
	validated     bool
}

// New creates a release discovery action.
func New(values ...Option) *Action {
	options := defaultOptions()
	for _, value := range values {
		if value != nil {
			value.ApplyTo(&options)
		}
	}

	action := &Action{fsys: options.FS, path: options.Path}
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
	if a.fsys == nil {
		return ErrFilesystemRequired
	}
	if a.path == "" {
		return ErrPathRequired
	}

	return nil
}

// Run reads and normalizes component release metadata.
func (a *Action) Run(_ context.Context) (api.ReleaseStatus, error) {
	err := a.Validate()
	if err != nil {
		return api.ReleaseStatus{}, err
	}

	data, err := fs.ReadFile(a.fsys, a.path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return api.ReleaseStatus{}, nil
		}
		return api.ReleaseStatus{}, fmt.Errorf("read release metadata %q: %w", a.path, err)
	}

	var status api.ReleaseStatus
	err = yaml.Unmarshal(data, &status)
	if err != nil {
		return api.ReleaseStatus{}, fmt.Errorf("parse release metadata %q: %w", a.path, err)
	}

	status.Releases = normalize(status.Releases)
	return status, nil
}
