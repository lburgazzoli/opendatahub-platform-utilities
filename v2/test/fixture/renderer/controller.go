// Package renderer demonstrates controller-owned manifest-kit rendering.
package renderer

import (
	"context"
	"errors"
	"fmt"

	manifestengine "github.com/k8s-manifest-kit/engine/pkg"
	manifestrender "github.com/k8s-manifest-kit/engine/pkg/render"
	manifesttypes "github.com/k8s-manifest-kit/engine/pkg/types"
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

var (
	ErrControllerRequired = errors.New("renderer controller is required")
	ErrRequestRequired    = errors.New("renderer request is required")
	ErrResourcesRequired  = errors.New("renderer resources accessor is required")
)

// ValuesFunc derives render values from the current reconciled instance.
type ValuesFunc func(api.PlatformObject) map[string]any

// Controller owns the renderer and the values derived from its module.
type Controller struct {
	renderer *manifestengine.Engine
	values   ValuesFunc
}

// NewController creates a controller-owned rendering example.
func NewController(renderer *manifestengine.Engine, values ValuesFunc) *Controller {
	return &Controller{
		renderer: renderer,
		values:   values,
	}
}

// RenderResources renders the controller's configured sources and publishes a
// complete snapshot only after the engine has succeeded.
func (c *Controller) RenderResources(ctx context.Context, request *pipeline.Request) error {
	if c == nil || c.renderer == nil {
		return ErrControllerRequired
	}

	if request == nil {
		return ErrRequestRequired
	}

	if request.Resources == nil {
		return ErrResourcesRequired
	}

	var values map[string]any
	if c.values != nil {
		values = c.values(request.Instance)
	}

	rendered, err := c.renderer.Render(ctx, manifestrender.WithValues(manifesttypes.Values(values)))
	if err != nil {
		return fmt.Errorf("render resources: %w", err)
	}

	objects := make(resources.List, len(rendered))
	copy(objects, rendered)

	request.Resources.Set(objects)

	return nil
}
