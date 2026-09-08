package manager

// Option configures the v2 manager wrapper.
type Option func(*Manager)

// WithManifestsBasePath configures the base path for manifests.
func WithManifestsBasePath(path string) Option {
	return func(manager *Manager) {
		manager.manifestsBasePath = path
	}
}

// WithChartsBasePath configures the base path for charts.
func WithChartsBasePath(path string) Option {
	return func(manager *Manager) {
		manager.chartsBasePath = path
	}
}
