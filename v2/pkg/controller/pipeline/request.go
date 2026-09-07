package pipeline

import (
	"github.com/opendatahub-io/odh-platform-utilities/v2/api"
	"github.com/opendatahub-io/odh-platform-utilities/v2/pkg/kube/resources"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Extension map[string]any

type Request struct {
	Client     client.Client
	Instance   api.PlatformObject
	Resources  resources.Accessor
	Extensions Extension
}
