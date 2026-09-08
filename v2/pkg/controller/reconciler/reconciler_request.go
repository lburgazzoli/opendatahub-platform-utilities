package reconciler

import (
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/controller/pipeline"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
)

func (r *Reconciler) request(instance api.PlatformObject) *pipeline.Request {
	return &pipeline.Request{
		Client:    r.client,
		Instance:  instance,
		Resources: resources.New(nil),
		Extensions: pipeline.Extension{
			pipeline.ExtensionControllerName: r.options.ControllerName,
			pipeline.ExtensionFieldOwner:     r.options.FieldOwner,
		},
	}
}
