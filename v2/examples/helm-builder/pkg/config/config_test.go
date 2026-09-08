package config_test

import (
	"testing"
	"testing/fstest"

	"github.com/onsi/gomega"

	moduleconfig "github.com/opendatahub-io/odh-platform-utilities/v2/examples/helm-builder/pkg/config"
)

func TestLoadFromFS(t *testing.T) {
	t.Parallel()

	g := gomega.NewWithT(t)
	loaded, err := moduleconfig.LoadFromFS(t.Context(), fstest.MapFS{
		moduleconfig.ChartPathConfigKey:              {Data: []byte("file-chart")},
		moduleconfig.NamespaceConfigKey:              {Data: []byte("file-namespace")},
		moduleconfig.HealthProbeBindAddressConfigKey: {Data: []byte("0")},
	})

	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(loaded.ChartPath).To(gomega.Equal("file-chart"))
	g.Expect(loaded.Namespace).To(gomega.Equal("file-namespace"))
	g.Expect(loaded.HealthProbeBindAddress).To(gomega.Equal("0"))
}

func TestEnvironmentOverridesFiles(t *testing.T) {
	t.Setenv(moduleconfig.ChartPathEnvVar, "environment-chart")
	t.Setenv(moduleconfig.NamespaceEnvVar, "environment-namespace")

	g := gomega.NewWithT(t)
	loaded, err := moduleconfig.LoadFromFS(t.Context(), fstest.MapFS{
		moduleconfig.ChartPathConfigKey: {Data: []byte("file-chart")},
		moduleconfig.NamespaceConfigKey: {Data: []byte("file-namespace")},
	})

	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(loaded.ChartPath).To(gomega.Equal("environment-chart"))
	g.Expect(loaded.Namespace).To(gomega.Equal("environment-namespace"))
	g.Expect(loaded.HealthProbeBindAddress).To(gomega.Equal(moduleconfig.DefaultHealthProbeBindAddress))
}

func TestLoadRequiresChartPathAndNamespace(t *testing.T) {
	t.Parallel()

	g := gomega.NewWithT(t)
	loaded, err := moduleconfig.LoadFromFS(t.Context(), nil)

	g.Expect(loaded).To(gomega.BeNil())
	g.Expect(err).To(gomega.MatchError(moduleconfig.ErrChartPathRequired))
}
