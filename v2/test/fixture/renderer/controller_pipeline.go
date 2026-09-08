package renderer

import "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"

// Action adapts the controller-owned render method to the pipeline.
func (c *Controller) Action() pipeline.Action {
	return pipeline.ActionFunc{
		ActionName:  "render-resources",
		ExecuteFunc: c.RenderResources,
	}
}
