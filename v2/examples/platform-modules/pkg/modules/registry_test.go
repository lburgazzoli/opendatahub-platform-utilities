package modules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/opendatahub-io/odh-platform-utilities/v2/examples/platform-modules/pkg/modules"
)

func TestLoadExampleBundles(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)

	registry, err := modules.Load(filepath.Join("..", "..", "config", "modules"))
	g.Expect(err).To(gomega.Succeed())
	g.Expect(registry.Names()).To(gomega.Equal([]string{"aigateway", "kserve"}))

	for _, name := range registry.Names() {
		definition, found := registry.Get(name)
		g.Expect(found).To(gomega.BeTrue())
		g.Expect(definition.Config.APIVersion).To(gomega.Equal(modules.ConfigAPIVersion))
		g.Expect(definition.Config.Kind).To(gomega.Equal(modules.ConfigKind))
		g.Expect(definition.Config.Metadata.Name).To(gomega.Equal(name))
		g.Expect(definition.Config.Spec.ModuleRef.Name).To(gomega.Equal("cluster"))
		g.Expect(filepath.Base(definition.Chart)).To(gomega.Equal("charts"))
	}

	kserve, _ := registry.Get("kserve")
	g.Expect(kserve.GVK().Kind).To(gomega.Equal("Kserve"))
	g.Expect(kserve.CRDName).To(gomega.Equal("kserves.kserve.example.odh.io"))
	g.Expect(kserve.Plural).To(gomega.Equal("kserves"))
}

func TestLoadConfigRejectsInvalidDocuments(t *testing.T) {
	t.Parallel()

	base := `apiVersion: deployer.opendatahub.io/v1alpha1
kind: PlatformModuleConfig
metadata:
  name: sample
spec:
  moduleRef:
    apiVersion: example.odh.io/v1alpha1
    kind: Sample
    name: cluster
`

	tests := []struct {
		name    string
		yaml    string
		message string
	}{
		{name: "unknown field", yaml: base + "unexpected: true\n", message: "field unexpected not found"},
		{name: "multiple documents", yaml: base + "---\n" + base, message: "exactly one YAML document"},
		{name: "invalid runlevel", yaml: base + "  runlevel: -1\n", message: "spec.runlevel"},
		{name: "duplicate service", yaml: base + "  services: [gateway, gateway]\n", message: "duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			g := gomega.NewWithT(t)

			_, err := modules.LoadConfig(strings.NewReader(tt.yaml))
			g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring(tt.message)))
		})
	}
}

func TestLoadRejectsChartOutsideModule(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	root := t.TempDir()
	directory := filepath.Join(root, "sample")
	g.Expect(os.Mkdir(directory, 0o750)).To(gomega.Succeed())

	config := `apiVersion: deployer.opendatahub.io/v1alpha1
kind: PlatformModuleConfig
metadata:
  name: sample
spec:
  moduleRef:
    apiVersion: sample.example.odh.io/v1alpha1
    kind: Sample
    name: cluster
  chart:
    path: ../outside
`
	g.Expect(os.WriteFile(filepath.Join(directory, "module.yaml"), []byte(config), 0o600)).To(gomega.Succeed())

	_, err := modules.Load(root)
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("chart path")))
}
