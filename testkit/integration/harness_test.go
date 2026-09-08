package integration_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/opendatahub-io/odh-platform-utilities/testkit/integration"
)

func TestDSCSpec_ToMap(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	spec := integration.NewDSCSpec().
		Component(integration.Dashboard, integration.Managed).
		Component(integration.Kserve, integration.Managed,
			integration.Sub("nim", integration.Managed),
			integration.Set("rawDeploymentServiceConfig", "Headed"),
		)

	m := spec.ToMap()
	g.Expect(m).Should(HaveKey("components"))

	components, ok := m["components"].(map[string]any)
	g.Expect(ok).Should(BeTrue(), "components must be map[string]any")

	g.Expect(components).Should(HaveKey("dashboard"))
	g.Expect(components).Should(HaveKey("kserve"))

	kserve, ok := components["kserve"].(map[string]any)
	g.Expect(ok).Should(BeTrue(), "kserve must be map[string]any")
	g.Expect(kserve["managementState"]).Should(Equal("Managed"))
	g.Expect(kserve["nim"]).Should(Equal(map[string]any{"managementState": "Managed"}))
	g.Expect(kserve["rawDeploymentServiceConfig"]).Should(Equal("Headed"))
}

func TestDSCSpec_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	g := NewWithT(t)
	spec := integration.NewDSCSpec()
	g.Expect(spec.ToMap()).Should(BeNil())
}
