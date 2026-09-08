// Package manager provides the v2 controller-runtime manager wrapper.
package manager

import (
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmanager "sigs.k8s.io/controller-runtime/pkg/manager"

	coherentclient "github.com/opendatahub-io/odh-platform-utilities/v2/pkg/client"
)

// Manager wraps a controller-runtime manager and exposes the cache-coherent
// v2 client to controllers and actions.
type Manager struct {
	ctrlmanager.Manager

	wrappedClient     *coherentclient.Client
	manifestsBasePath string
	chartsBasePath    string
}

// New wraps an existing controller-runtime manager.
func New(inner ctrlmanager.Manager, options ...Option) *Manager {
	wrapped := &Manager{
		Manager:       inner,
		wrappedClient: coherentclient.New(inner.GetClient()),
	}

	for _, option := range options {
		if option != nil {
			option(wrapped)
		}
	}

	return wrapped
}

func (m *Manager) GetClient() client.Client {
	return m.wrappedClient
}

func (m *Manager) GetManifestsBasePath() string {
	return m.manifestsBasePath
}

func (m *Manager) GetChartsBasePath() string {
	return m.chartsBasePath
}

var _ ctrlmanager.Manager = (*Manager)(nil)
