package modules_test

import (
	"os"
	"path/filepath"
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

	kserve, found := registry.Get("kserve")
	g.Expect(found).To(gomega.BeTrue())
	g.Expect(kserve.CRD).To(gomega.Equal("kserves.kserve.example.odh.io"))
	g.Expect(kserve.GVK().Kind).To(gomega.Equal("Kserve"))
	g.Expect(kserve.Resource()).To(gomega.Equal("kserves"))
	g.Expect(filepath.Base(kserve.Chart)).To(gomega.Equal("module-controller"))
}

func TestLoadRejectsInconsistentCRD(t *testing.T) {
	t.Parallel()
	g := gomega.NewWithT(t)
	root := t.TempDir()
	bundle := filepath.Join(root, "invalid")
	g.Expect(os.Mkdir(bundle, 0o750)).To(gomega.Succeed())
	g.Expect(os.WriteFile(filepath.Join(bundle, "module.yaml"), []byte(`
name: invalid
crd: invalid.other.example
apiVersion: example.platform.odh.io/v1alpha1
kind: Invalid
chart: chart
`), 0o600)).To(gomega.Succeed())

	_, err := modules.Load(root)
	g.Expect(err).To(gomega.MatchError(gomega.ContainSubstring("must use group")))
}
